分享一个简单的`operator`的编写

> 为了理解的更加深刻, 以下案例不使用`kuberbuilder`, 只使用了`code-generator`, 代码的编写参考了极客时间张磊老师的`深入剖析 Kubernetes`和https://github.com/kubernetes/sample-controller

### 需求

* 定义一个名为`App`的`crd`, 当`crd`被创建出来的时候, 自动创建对应的`deployment`和`service`

### 实现

#### 工具准备

  *  准备`code-generator`
  
     ```shell
     # 把code-generator先clone下来
     cd ~/go/src/github.com
     git clone https://github.com/kubernetes/code-generator.git
     ```



#### 创建工程

  *  在`GOPATH`路径下创建项目`github.com/my-operator/apps`, 目录如下:
  
     ```shell
     root@node1:~/go/src/github.com/apps# tree .
     .
     ├── controller.go
     ├── main.go
     ├── manifests
     │   ├── crd
     │   │   └── app-crd.yaml
     │   └── example
     │       └── example-app.yaml
     └── pkg
         └── apis
             └── myapp
                 ├── register.go
                 └── v1
                     ├── doc.go
                     ├── register.go
                     └── types.go
     
     ```
  
     
  
  *  使用`go moudle`初始化项目
  
     ```shell
     cd ~/go/src/github.com/my-operator/apps
     go mod init "github.com/my-operator/apps"
     ```



#### 编写代码

##### 资源定义

*  编写 `manifests/crd/app-crd.yaml`, 定义`crd`

    ```yaml
    apiVersion: apiextensions.k8s.io/v1
    kind: CustomResourceDefinition
    metadata:
      name: apps.myapp.k8s.io
      annotations:
        "api-approved.kubernetes.io": "unapproved, experimental-only; please get an approval from Kubernetes API reviewers if you're trying to develop a CRD in the *.k8s.io or *.kubernetes.io groups"
    spec:
      group: myapp.k8s.io
      versions:
        - name: v1
          served: true
          storage: true
          schema:
            openAPIV3Schema:
              type: object
              properties:
                spec:
                  type: object
                  properties:
                    deployment:
                      type: object
                      properties:
                        name:
                          type: string
                        image: 
                          type: string
                        replicas:
                          type: integer
                          format: int32
                    service:
                      type: object
                      properties:
                        name:
                          type: string
                status:
                  type: object
                  properties:
                    availableReplicas:
                      type: integer
      names:
        kind: App
        plural: apps
      scope: Namespaced
    ```

    

*  `manifests/example/example-app.yaml`

    ```yaml
    apiVersion: myapp.k8s.io/v1
    kind: App
    metadata:
      name: example-app
    spec:
      deployment:
        name: example-app
        image: nginx:latest
        replicas: 2
      service:
        name: example-app
    
    ```

    

*  `pkg/apis/myapp/register.go` ，用来放置后面要用到的全局变量

   ```go
   package myapp
   
   const (
   	GroupName = "myapp.k8s.io"
   	Version   = "v1"
   )
   
   ```

   

*  `apps/pkg/myapp/v1/doc.go`

   ```go
   // +k8s:deepcopy-gen=package
   // +groupName=myapp.k8s.io
   package v1
   
   ```



* `apps/pkg/myapp/v1/types.go`

  ```go
  package v1
  
  import (
  	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  )
  
  // +genclient
  // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
  
  // App is a specification for a App resource
  type App struct {
  	metav1.TypeMeta   `json:",inline"`
  	metav1.ObjectMeta `json:"metadata,omitempty"`
  
  	Spec   AppSpec   `json:"spec"`
  	Status AppStatus `json:"status"`
  }
  
  type ServiceSpec struct {
  	Name string `json:"name"`
  }
  
  type DeploymentSpec struct {
  	Name     string `json:"name"`
  	Image    string `json:"image"`
  	Replicas int32  `json:"replicas"`
  }
  
  // AppSpec is the spec for a Foo resource
  type AppSpec struct {
  	Deployment DeploymentSpec `json:"deployment"`
  	Service    ServiceSpec    `json:"service"`
  }
  
  // AppStatus is the status for a Foo resource
  type AppStatus struct {
  	AvailableReplicas int32 `json:"availableReplicas"`
  }
  
  // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
  
  type AppList struct {
  	metav1.TypeMeta `json:",inline"`
  	metav1.ListMeta `json:"metadata"`
  
  	Items []App `json:"items"`
  }
  
  ```

  

* `pkg/apis/myapp/v1/register.go`

  ```go
  package v1
  
  import (
  	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  	"k8s.io/apimachinery/pkg/runtime"
  	"k8s.io/apimachinery/pkg/runtime/schema"
  
  	apps "github.com/my-operator/apps/pkg/apis/myapp"
  )
  
  // SchemeGroupVersion is group version used to register these objects
  var SchemeGroupVersion = schema.GroupVersion{Group: apps.GroupName, Version: apps.Version}
  
  // Kind takes an unqualified kind and returns back a Group qualified GroupKind
  func Kind(kind string) schema.GroupKind {
  	return SchemeGroupVersion.WithKind(kind).GroupKind()
  }
  
  // Resource takes an unqualified resource and returns a Group qualified GroupResource
  func Resource(resource string) schema.GroupResource {
  	return SchemeGroupVersion.WithResource(resource).GroupResource()
  }
  
  var (
  	// SchemeBuilder initializes a scheme builder
  	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
  	// AddToScheme is a global function that registers this API group & version to a scheme
  	AddToScheme = SchemeBuilder.AddToScheme
  )
  
  // Adds the list of known types to Scheme.
  func addKnownTypes(scheme *runtime.Scheme) error {
  	scheme.AddKnownTypes(SchemeGroupVersion,
  		&App{},
  		&AppList{},
  	)
  	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
  	return nil
  }
  
  ```

* 使用`code-generator`

  ```shell
  # 必须在项目路径下, 否则导包会有问题
  cd ~/go/src/github.com/my-operator/apps
  
  # 执行go mod tidy, 下载缺失的依赖
  go mod tidy
  
  # code-generator代码路径
  CODE_GENERATOR_PATH="/root/go/src/github.com/code-generator"
  
  # 代码生成的工作目录，也就是我们的项目路径($GOPATH后面的部分), 同时也应该是我们的包名, 否则会失败
  ROOT_PACKAGE="github.com/my-operator/apps"
  
  # API Group
  CUSTOM_RESOURCE_NAME="myapp"
  # API Version
  CUSTOM_RESOURCE_VERSION="v1"
  
  # output-base的位置也要注意, 是要从当前位置一直..到GOPATH的路径, 否则生成的代码的路径会跳到别的地方, -v 10是用来查看脚本运行信息的, 不加该参数的话, 看不到具体的报错信息
  "$CODE_GENERATOR_PATH"/generate-groups.sh all "$ROOT_PACKAGE/pkg/client" "$ROOT_PACKAGE/pkg/apis" "$CUSTOM_RESOURCE_NAME:$CUSTOM_RESOURCE_VERSION" --go-header-file="$CODE_GENERATOR_PATH"/hack/boilerplate.go.txt --output-base ../../..  -v 10
  
  # 代码生成成功后的文件目录结构如下
  root@node1:~/go/src/github.com/my-operator/apps# tree .
  .
  ├── controller.go
  ├── go.mod
  ├── go.sum
  ├── main.go
  ├── manifests
  │   ├── crd
  │   │   └── app-crd.yaml
  │   └── example
  │       └── example-app.yaml
  └── pkg
      ├── apis
      │   └── myapp
      │       ├── register.go
      │       └── v1
      │           ├── doc.go
      │           ├── register.go
      │           ├── types.go
      │           └── zz_generated.deepcopy.go
      └── client
          ├── clientset
          │   └── versioned
          │       ├── clientset.go
          │       ├── doc.go
          │       ├── fake
          │       │   ├── clientset_generated.go
          │       │   ├── doc.go
          │       │   └── register.go
          │       ├── scheme
          │       │   ├── doc.go
          │       │   └── register.go
          │       └── typed
          │           └── myapp
          │               └── v1
          │                   ├── app.go
          │                   ├── doc.go
          │                   ├── fake
          │                   │   ├── doc.go
          │                   │   ├── fake_app.go
          │                   │   └── fake_myapp_client.go
          │                   ├── generated_expansion.go
          │                   └── myapp_client.go
          ├── informers
          │   └── externalversions
          │       ├── factory.go
          │       ├── generic.go
          │       ├── internalinterfaces
          │       │   └── factory_interfaces.go
          │       └── myapp
          │           ├── interface.go
          │           └── v1
          │               ├── app.go
          │               └── interface.go
          └── listers
              └── myapp
                  └── v1
                      ├── app.go
                      └── expansion_generated.go
  
  ```
  
  

##### 控制器逻辑

* `main.go`

  ```go
  package main
  
  import (
  	"flag"
  	"time"
  
  	kubeInformers "k8s.io/client-go/informers"
  	kubeClientset "k8s.io/client-go/kubernetes"
  	"k8s.io/client-go/tools/clientcmd"
  	"k8s.io/klog/v2"
  
  	appClientset "github.com/my-operator/apps/pkg/client/clientset/versioned"
  	appInformers "github.com/my-operator/apps/pkg/client/informers/externalversions"
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
  
  ```

  

* `controller.go`

  ```go
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
  
  	appV1 "github.com/my-operator/apps/pkg/apis/myapp/v1"
  	myappClientset "github.com/my-operator/apps/pkg/client/clientset/versioned"
  	"github.com/my-operator/apps/pkg/client/clientset/versioned/scheme"
  	myappInformers "github.com/my-operator/apps/pkg/client/informers/externalversions/myapp/v1"
  	myappListers "github.com/my-operator/apps/pkg/client/listers/myapp/v1"
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
  
  ```



##### 调试

* 先创建出相应的资源

  ```shell
  kubectl apply -f manifests/crd/
  kubectl apply -f manifests/example/
  ```

* 本地调试

  > 在主节点上直接运行

  ```shell
  go mod tidy
  go run . --kubeconfig=/root/.kube/config
  ```

* 如果需要部署到集群的话, 需要创建出相应的`rbac`, 否则会因为权限读不到`api-server`的数据
