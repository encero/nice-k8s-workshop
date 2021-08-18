cat /var/lib/rancher/k3s/server/node-token

echo "curl -sfL https://get.k3s.io | K3S_URL=https://$(hostname --fqdn):6443 K3S_TOKEN=$(cat /var/lib/rancher/k3s/server/node-token) sh -"
