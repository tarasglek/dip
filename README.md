# Purpose
This tool sets up dns for apiserver node, ingress entries + rest of kubernetes nodes.
This is especially useful when switching between multiple clusters as it avoids need to manually edit /etc/hosts, deal with stale entries 

# Compile
make && sudo make install

# Usage

`sudo k8s-hosts-sync -controller_ip 10.15.216.177 --kubeconfig ~/.kube/config`
In this case we:
1. Read dns hostname from .kube/config(eg controller1.example.com)
2. Write/replace '10.15.216.177 controller1.example.com' to /etc/hosts
3. Connect to kubernetes, enumerate ingresses, nodes
4. Write out entries from #3 to /etc/hosts

# Todo
Figure out how to pull certificates and install them into local cert store