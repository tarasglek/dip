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

	for host, namespace := range *hosts {
		fmt.Printf("'%s' in '%s'\n", host, namespace)
		_, writeErr := writer.WriteString(*ingressIP + " " + host + mySuffix)
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
	return os.Rename(newFileName, *hostsFile)
}

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	ingressControllerIP := flag.String("ingress_controller_ip", "", "IP address the resolve ingress names to. Leave empty to resolve this is same as kubeconfig hostname")
	hostsFile := flag.String("hosts_file", "/etc/hosts", "Ip address to redirect to")
	runForever := flag.Bool("run-forever", true, "Continuously poll kubeconfig & update /etc/hosts")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	if *ingressControllerIP == "" {
		controllerURL, err := url.Parse(config.Host)
		if err != nil {
			panic(err.Error())
		}

		host, _, _ := net.SplitHostPort(controllerURL.Host)

		ips, err := net.LookupHost(host)
		if err != nil {
			panic(err.Error())
		}
		ingressControllerIP = &ips[0]
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	oldHosts := map[string]string{}
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
					newHosts[rule.Host] = namespace.Name
				}
			}
		}
		if !reflect.DeepEqual(oldHosts, newHosts) {
			oldHosts = newHosts
			err := updateHosts(&oldHosts, ingressControllerIP, hostsFile)
			if err != nil {
				panic(err.Error())
			}
			fmt.Printf("Wrote out %d ingresses to %s\n", len(newHosts), *hostsFile)
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
