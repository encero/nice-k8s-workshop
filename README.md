# Pred workshopem 

- jste zapsani v tabulce abych vam vyrobil VPS na ktere to cele budem zkouset
- naklonujte si tohle repo
- cca hodinu pred workshopem uz budou nastartovane VPSky zkuste se na ni pripojit pres `ssh root@<vase-user-name-z-tabulky>.encero.xyz`
- mate naistalovany kubectl https://kubernetes.io/docs/tasks/tools/
- nebo vam bezi docker image zmineny nize (over `kubectl version`)


# install k3s

```sh
curl -sfL https://get.k3s.io | sh -
```

# verify k3s is running
```sh
k3s kubectl get node --watch
k3s kubectl get pods --all-namespaces
```


# get kube config
```sh
scp root@encero.encero.xyz:/etc/rancher/k3s/k3s.yaml ./
export KUBECONIG=$PWD/k3s.yaml
```

# install kubectl localy
https://kubernetes.io/docs/tasks/tools/

# OR use docker
```sh
docker build -t kube .

docker run --rm -it -v $(pwd):/workshop kube
# or
./kube.sh
```

# hello app
```sh
kubectl apply -f yamls/hello.yaml

kubectl port-forward <pod-name> 8080:8080
kubectl logs -f <pod-name>
```

# hello app with ingress
```sh
kubectl apply -f yamls/hello-ingress.yaml
```

# install cert manager

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
```

# hello app with TLS

```sh
kubectl apply -f yamls/hello-ingress-tls.yaml
```
