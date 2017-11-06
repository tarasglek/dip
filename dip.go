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

func updateHosts(hosts *map[string]string, ingressIP *string, hostsFile *string) error {
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

	for host, namespace := range *hosts {
		fmt.Printf("'%s' in '%s'\n", host, namespace)
		writer.WriteString(*ingressIP + " " + host + mySuffix)
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return os.Rename(newFileName, *hostsFile)
}

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	ingressIP := flag.String("ingress_host", "127.0.0.1", "Ip address to redirect to")
	hostsFile := flag.String("hosts_file", "/etc/hosts", "Ip address to redirect to")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	old_hosts := map[string]string{}
	for {
		new_hosts := map[string]string{}

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
					new_hosts[rule.Host] = namespace.Name
				}
			}
		}
		if !reflect.DeepEqual(old_hosts, new_hosts) {
			old_hosts = new_hosts
			err := updateHosts(&old_hosts, ingressIP, hostsFile)
			if err != nil {
				panic(err.Error())
			}
			fmt.Printf("Wrote out %d ingresses to %s\n", len(new_hosts), *hostsFile)
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
