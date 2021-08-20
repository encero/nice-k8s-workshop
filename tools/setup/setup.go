package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/digitalocean/godo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const sshFingerprint = "5c:48:95:fa:ec:f4:3c:76:78:f2:77:1b:ad:a5:7c:d4"
const region = "fra1"
const distribution = "ubuntu-21-04-x64"

const size = "s-2vcpu-4gb-amd"
const dropletTag = "nice-workshop"
const domain = "encero.xyz"

const defaultKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHfXEOvy8FgUbO4Wile2w1M9p575UUltJGqZ9MvOtrpl"

var debug = flag.Bool("debug", false, "verbose output")
var dryrun = flag.Bool("dry", false, "dry run")
var userDataTpl = template.Must(template.ParseGlob("userdata.yaml"))

type user struct {
	name     string
	hostname string
	Keys     []string
}

func main() {
	doToken := os.Getenv("DO_TOKEN")
	if doToken == "" {
		fmt.Println("DO_TOKEN env missing")
		os.Exit(1)
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Usage: go run setup.go [setup, teardown]\n")
		os.Exit(1)
	}

	setupLogger()

	zap.S().Infof("dryrun: %v debug: %v", *dryrun, *debug)

	client := godo.NewFromToken(doToken)

	switch strings.ToLower(args[0]) {
	case "setup":
		runSetup(client)
	case "teardown":
		runTeardown(client)
	}
}

func setupLogger() {
	c := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	c.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	if *debug {
		c.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	l, _ := c.Build()
	zap.ReplaceGlobals(l)
}

func runSetup(client *godo.Client) {
	zap.S().Info("running workshop setup...")

	users, err := fetchUsers()
	if err != nil {
		zap.L().Panic(err.Error())
	}

	zap.S().Infof("found %d users in configuration", len(users))

	currentDroplets, err := listDroplets(client)
	if err != nil {
		zap.L().Panic(err.Error())
	}

	if len(currentDroplets) > 0 {
		zap.S().Infof("found %d droplets already registered", len(currentDroplets))
	}

	missingUser := dropletDiff(users, currentDroplets)

	if len(missingUser) == 0 {
		zap.S().Infof("All droplets already created")
	} else {
		zap.S().Infof("%d users dont have their droplets", len(missingUser))

		createUserDroplets(client, missingUser)

		zap.S().Info("waiting for droplets to startup")
		time.Sleep(time.Second * 30)
	}

	currentDroplets, err = listDroplets(client)
	if err != nil {
		zap.L().Panic(err.Error())
	}

	records, resp, err := client.Domains.RecordsByType(context.Background(), domain, "A", &godo.ListOptions{PerPage: 200})
	if err != nil {
		zap.S().Panic(err.Error())
	}
	reportReachingLimit(resp)

	recordMap := make(map[string]godo.DomainRecord, len(records))
	for _, re := range records {
		recordMap[re.Name] = re
	}

	dropletNameSuffix := "." + domain

	for _, d := range currentDroplets {
		addr, err := d.PublicIPv4()
		if err != nil {
			zap.S().Errorf("missing public IPv4 adress for droplet %s", d.Name)
			continue
		}

		subdomain := strings.TrimSuffix(d.Name, dropletNameSuffix)
		wildcard := "*." + subdomain

		updateRecord(client, subdomain, recordMap, addr)
		updateRecord(client, wildcard, recordMap, addr)
	}
}

func updateRecord(client *godo.Client, subdomain string, records map[string]godo.DomainRecord, addr string) {
	re, ok := records[subdomain]
	if !ok {
		err := createDomainRecord(client, subdomain, addr)
		if err != nil {
			zap.S().Error(err.Error())
		}

		zap.S().Infof("created domain %s %s", subdomain, addr)
		return
	}

	if re.Data == addr {
		zap.S().Infof("no domain change %s", subdomain)
		return
	}

	_, resp, err := client.Domains.EditRecord(context.Background(), domain, re.ID, &godo.DomainRecordEditRequest{
		Data: addr,
	})
	reportReachingLimit(resp)
	if err != nil {
		zap.S().Error(err.Error())
		return
	}

	zap.S().Infof("changed domain record for %s %s -> %s", subdomain, re.Data, addr)
}

func runTeardown(client *godo.Client) {
	zap.S().Infof("removing all droplets with tag %q", dropletTag)

	destroyDroplets(client)
}

func createUserDroplets(client *godo.Client, hostnames []user) {
	for _, user := range hostnames {
		createDroplet(client, user)
		zap.S().Infof("created droplet for hostname: %s", user.hostname)
	}
}

func dropletDiff(wantUsers []user, currentDroplets []godo.Droplet) []user {
	dropMap := make(map[string]godo.Droplet, len(currentDroplets))
	for _, d := range currentDroplets {
		dropMap[d.Name] = d
	}

	missing := make([]user, 0, len(wantUsers))

	for _, usr := range wantUsers {
		_, ok := dropMap[usr.hostname]
		if ok {
			continue
		}

		missing = append(missing, usr)
	}

	return missing
}

func dropletHostnameFromName(name string) string {
	return fmt.Sprintf("%s.%s", name, domain)
}

func fetchUsers() ([]user, error) {
	contents, err := ioutil.ReadFile("users.list")
	if err != nil {
		return nil, fmt.Errorf("reading users: %w", err)
	}

	var users []user

	buf := bytes.NewBuffer(contents)

	for {
		line, err := buf.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("reading lines from users: %w", err)
		}

		if line != "" {
			users = append(users, makeUser(line))
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return users, nil
}

func makeUser(line string) user {
	usr := user{}

	parts := strings.Split(line, ";")

	usr.name = normalizeName(parts[0])
	usr.hostname = dropletHostnameFromName(usr.name)

	key := strings.TrimSpace(parts[1])
	if len(key) > 0 {
		usr.Keys = append(usr.Keys, key)
	}

	ghKeys := loadGithubKeys(usr.name)
	if len(ghKeys) > 0 {
		usr.Keys = append(usr.Keys, ghKeys...)
	}

	usr.Keys = append(usr.Keys, defaultKey)

	return usr
}

func loadGithubKeys(name string) []string {
	resp, err := http.Get(fmt.Sprintf("https://github.com/%s.keys", name))
	if err != nil {
		zap.S().Errorf("cant load github keys for user %s err: %s", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		zap.S().Errorf("cant load github keys for user %s non 200 code", name)
		return nil
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		zap.S().Errorf("cant load github keys for user %s err: %s", name, err)
	}

	var keys []string
	for _, k := range strings.Split(string(data), "\n") {
		k := strings.TrimSpace(k)

		if k != "" {
			keys = append(keys, k)
		}
	}

	return keys
}

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, ".", "-")

	return name
}

func destroyDroplets(client *godo.Client) {
	_, err := client.Droplets.DeleteByTag(context.Background(), dropletTag)
	if err != nil {
		panic(err)
	}
}

func listDroplets(client *godo.Client) ([]godo.Droplet, error) {
	droplets, res, err := client.Droplets.ListByTag(context.Background(), dropletTag, &godo.ListOptions{
		PerPage: 200,
	})

	if err != nil {
		return nil, fmt.Errorf("list DO droplets: %w", err)
	}

	reportReachingLimit(res)

	return droplets, err
}

func reportReachingLimit(res *godo.Response) {
	if res == nil {
		return
	}

	if res.Remaining < 200 {
		fmt.Printf("Reaching DO request limit, remainging: %d", res.Remaining)
	}
}

func createDroplet(client *godo.Client, usr user) {
	usrData := &bytes.Buffer{}
	if err := userDataTpl.Execute(usrData, usr); err != nil {
		zap.S().Error("user data template failure: %s", err)
		return
	}

	if *debug {
		fmt.Println("---- usr data:", usr.hostname)
		fmt.Println(usrData.String())
		fmt.Println("----")
	}

	req := &godo.DropletCreateRequest{
		Name:   usr.hostname,
		Region: region,
		Size:   size,
		Image: godo.DropletCreateImage{
			Slug: distribution,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				Fingerprint: sshFingerprint,
			},
		},
		Tags:     []string{dropletTag},
		UserData: usrData.String(),
	}

	if *dryrun {
		zap.S().Info("Dry run: creating droplet for user")
		spew.Dump(req)
		return
	}

	_, resp, err := client.Droplets.Create(context.Background(), req)

	reportReachingLimit(resp)

	if err != nil {
		panic(err)
	}
}

func createDomainRecord(client *godo.Client, subdomain, ip string) error {
	_, resp, err := client.Domains.CreateRecord(context.Background(), "encero.xyz", &godo.DomainRecordEditRequest{
		Type: "A",
		Name: subdomain,
		Data: ip,
		TTL:  30,
	})

	reportReachingLimit(resp)

	return err
}
