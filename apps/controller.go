package main

import (
	"context"
	"fmt"
	"time"

	appApi "k8s.io/api/apps/v1"
	coreApi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	kubeClientset "k8s.io/client-go/kubernetes"
	appsListers "k8s.io/client-go/listers/apps/v1"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	appV1 "github.com/cloudnative/summarize/apps/pkg/apis/myapp/v1"
	myappClientset "github.com/cloudnative/summarize/apps/pkg/client/clientset/versioned"
	"github.com/cloudnative/summarize/apps/pkg/client/clientset/versioned/scheme"
	myappInformers "github.com/cloudnative/summarize/apps/pkg/client/informers/externalversions/myapp/v1"
	myappListers "github.com/cloudnative/summarize/apps/pkg/client/listers/myapp/v1"
)

const (
	maxRetry = 10
)

// controller数据结构
type Controller struct {
	kubeClient        kubeClientset.Interface
	appClinet         myappClientset.Interface
	deploymentLister  appsListers.DeploymentLister
	deploymentsSynced cache.InformerSynced
	serviceLister     coreListers.ServiceLister
	serviceSynced     cache.InformerSynced
	appLister         myappListers.AppLister
	appsSynced        cache.InformerSynced
	queue             workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder
}

// 入workqueue方法
func (c *Controller) enqueueApp(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	// 把对象的key放到workqueue里面
	c.queue.Add(key)
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer c.queue.ShutDown()
	klog.Info("Starting App controller")

	// // 等待缓存同步
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.appsSynced, c.deploymentsSynced, c.serviceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// worker开始工作
	klog.Info("Starting workers")
	// Launch two workers to process App resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// worker
func (c *Controller) worker() {
	for c.processNextItem() {
	}
}

// 单个执行周期
func (c *Controller) processNextItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	key := obj.(string)

	if err := c.syncHandler(key); err != nil {
		c.handlerError(key, err)
	}
	c.queue.Forget(obj)

	return true
}

// 错误处理
func (c *Controller) handlerError(key string, err error) {
	// 小于10次的时候重新入队
	if c.queue.NumRequeues(key) <= maxRetry {
		c.queue.AddRateLimited(key)
		return
	}
	runtime.HandleError(err)
	c.queue.Forget(key)
}

// 真正的处理逻辑
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// 获取app对象
	app, err := c.appLister.Apps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	deployment, err := c.deploymentLister.Deployments(app.Namespace).Get(app.Spec.Deployment.Name)
	// 如果没有找到对应的deployment, 创建deployment
	if errors.IsNotFound(err) {
		deployment, err = c.kubeClient.AppsV1().Deployments(app.Namespace).Create(context.TODO(), newDeployment(app), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	// 如果没有找到对应的service, 创建service
	service, err := c.serviceLister.Services(app.Namespace).Get(app.Spec.Service.Name)
	if errors.IsNotFound(err) {
		service, err = c.kubeClient.CoreV1().Services(app.Namespace).Create(context.TODO(), newService(app), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	// 如果找到了不是属于App控制的资源, 返回错误信息
	if !metav1.IsControlledBy(deployment, app) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by App", deployment.Name)
		c.recorder.Event(app, coreApi.EventTypeWarning, "ErrResourceExists", msg)
		return fmt.Errorf("%s", msg)
	}
	if !metav1.IsControlledBy(service, app) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by App", service.Name)
		c.recorder.Event(app, coreApi.EventTypeWarning, "ErrResourceExists", msg)
		return fmt.Errorf("%s", msg)
	}

	c.recorder.Event(app, coreApi.EventTypeNormal, "Synced", "App synced successfully")
	return nil

}

// deployment对象创建
func newDeployment(app *appV1.App) *appApi.Deployment {
	labels := map[string]string{
		"app":        "myApp",
		"controller": app.Name,
	}
	return &appApi.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Deployment.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, appV1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: appApi.DeploymentSpec{
			Replicas: &app.Spec.Deployment.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: coreApi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: coreApi.PodSpec{
					Containers: []coreApi.Container{
						{
							Name:  app.Spec.Deployment.Name,
							Image: app.Spec.Deployment.Image,
						},
					},
				},
			},
		},
	}
}

// service对象创建
func newService(app *appV1.App) *coreApi.Service {
	labels := map[string]string{
		"app":        "myApp",
		"controller": app.Name,
	}
	return &coreApi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Service.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, appV1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: coreApi.ServiceSpec{
			Selector: labels,
			Ports: []coreApi.ServicePort{
				{
					Protocol:   coreApi.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
			},
		},
	}
}

// Controller构造函数
func NewController(
	kubeClient kubeClientset.Interface,
	appClinet myappClientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	serviceInformer coreinformers.ServiceInformer,
	appInformer myappInformers.AppInformer,
) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, coreApi.EventSource{Component: "apps"})

	c := Controller{
		kubeClient:        kubeClient,
		appClinet:         appClinet,
		deploymentLister:  deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		serviceSynced:     serviceInformer.Informer().HasSynced,
		appLister:         appInformer.Lister(),
		appsSynced:        appInformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Apps"),
		recorder:          recorder,
	}

	// 注册EventHandler
	appInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueApp,
		UpdateFunc: func(old, new interface{}) {
			c.enqueueApp(new)
		},
	})

	return &c
}
