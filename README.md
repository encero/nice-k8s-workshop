# Steps to run k3s cluster on own hw


master:
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--flannel-backend=none --disable-network-policy' sh -

run on master, execute on agents
echo "curl -sfL https://get.k3s.io | K3S_URL=https://$(hostname --fqdn):6443 K3S_TOKEN=$(cat /var/lib/rancher/k3s/server/node-token) sh -"

# install cilium, flannel is not behaving nicely for some reason
https://docs.cilium.io/en/stable/gettingstarted/k3s/

cilium install

cilium status --wait

#install cert manager

kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml

kubectl apply -f yamls/cert-manger.yaml

# deploy hello app, service and ingress

kubectl apply -f yamls/hello.yaml

kubectl get pod --watch

# deploy wordpress

kubectl apply -f yamls/wordpress-deployment.yaml -f yamls/mysql-deployment.yaml

