package main

import (
	"flag"
	"time"

	kubeInformers "k8s.io/client-go/informers"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	appClientset "github.com/cloudnative/summarize/apps/pkg/client/clientset/versioned"
	appInformers "github.com/cloudnative/summarize/apps/pkg/client/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// 创建操作k8s内建资源的client
	kubeClient, err := kubeClientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	// 创建操作自定义资源app的client
	appClient, err := appClientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building app clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeInformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	appInformerFactory := appInformers.NewSharedInformerFactory(appClient, time.Second*30)

	controller := NewController(
		kubeClient,
		appClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Core().V1().Services(),
		appInformerFactory.Myapp().V1().Apps(),
	)

	stopCh := make(chan struct{})
	kubeInformerFactory.Start(stopCh)
	appInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
