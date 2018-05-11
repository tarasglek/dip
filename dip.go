/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note: the example only works with the code within the same release/branch.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func updateHosts(hosts *map[string]string, hostsFile *string) error {
	mySuffix := " # devingressproxy\n"
	newFileName := *hostsFile + ".new"
	inFile, err := os.Open(*hostsFile)
	defer inFile.Close()
	if err != nil {
		return err
	}

	outFile, err := os.Create(newFileName)
	defer outFile.Close()
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(outFile)

	for host, ip := range *hosts {
		fmt.Printf("%s -> %s\n", host, ip)
		_, writeErr := writer.WriteString(ip + " " + host + mySuffix)
		if writeErr != nil {
			return writeErr
		}
	}
	// Start reading from the file with a reader.
	reader := bufio.NewReader(inFile)

	var line string
	for {
		line, err = reader.ReadString('\n')

		if !strings.Contains(line, mySuffix) {
			// Process the line here.
			_, writeErr := writer.WriteString(line)
			if writeErr != nil {
				return writeErr
			}
		}

		if err != nil {
			break
		}
	}

	if err != io.EOF {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}
	err = os.Rename(newFileName, *hostsFile)
	if err != nil {
		fmt.Printf("Wrote out %d addresses to %s\n", len(*hosts), *hostsFile)
	}
	return err
}

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	ingressControllerIP := flag.String("controller_ip", "", "Force ingress to resolve to this ip. Will add this to hosts file to match .kube/config")
	hostsFile := flag.String("hosts_file", "/etc/hosts", "/etc/hosts or equivalent file")
	ipType := flag.String("ip_type", "InternalIP", "IP to pull out of kubernetes. Either InternalIP or ExternalIP")
	runForever := flag.Bool("run-forever", false, "Continuously poll kubeconfig & update /etc/hosts")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	controllerURL, err := url.Parse(config.Host)
	if err != nil {
		panic(err.Error())
	}
	host, _, err := net.SplitHostPort(controllerURL.Host)
	if err != nil {
		host = controllerURL.Host
	}

	oldHosts := map[string]string{}
	if *ingressControllerIP == "" {
		fmt.Printf("Looking up '%s' from %s\n", host, config.Host)
		ips, err := net.LookupHost(host)
		if err != nil {
			panic(err.Error())
		}
		ingressControllerIP = &ips[0]
	} else {
		oldHosts[host] = *ingressControllerIP
		err := updateHosts(&oldHosts, hostsFile)
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		newHosts := map[string]string{}

		namespaces, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		for _, namespace := range namespaces.Items {
			ingresses, err := clientset.ExtensionsV1beta1().Ingresses(namespace.Name).List(metav1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}
			for _, ingress := range ingresses.Items {
				for _, rule := range ingress.Spec.Rules {
					newHosts[rule.Host] = *ingressControllerIP
				}
			}
		}
		nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
		for _, node := range nodes.Items {
			nodeIP := ""
			nodeHostname := ""
			for _, address := range node.Status.Addresses {
				if string(address.Type) == *ipType {
					nodeIP = address.Address
				} else if address.Type == "Hostname" {
					nodeHostname = address.Address
				}
			}
			if len(nodeIP) > 0 && nodeIP != nodeHostname {
				newHosts[nodeHostname] = nodeIP
			}
		}

		if !reflect.DeepEqual(oldHosts, newHosts) {
			oldHosts = newHosts
			err := updateHosts(&oldHosts, hostsFile)
			if err != nil {
				panic(err.Error())
			}
		}
		if *runForever == false {
			break
		}
		time.Sleep(10 * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
