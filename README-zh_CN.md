<div align="center">

[English](./README.md) | 简体中文

</div>

# Module Controller

[![codecov](https://codecov.io/github/koupleless/module-controller/graph/badge.svg?token=L1SvoSDQsw)](https://codecov.io/github/koupleless/module-controller)

低成本地让开源用户接入模块运维体系。

# 架构设计

## 整体思路
整体思想是用 mock 的方式利用 k8s 现有的的运维调度能力，完成低成本的模块运维体系接入。

### 如何触发调度？
把基座 pod mock 成一个 k8s 的 node 节点，把模块 mock 成 k8s 的一个 pod，由此 kube-scheduler 会触发一轮调度，并且给 pod 分配上一个合适的 node。<br />值的注意的是，mock 的 pod 只能被调度到 mock 的 node 上，否则会造成无法执行安装的异常。这是因为，正常的 node 节点上的 kubelet 只会执行 pod 的安装流程。只有 mock 的特殊 node 上才存在 virtual-kubelet，只有 virtual-kubelet 会识别到模块，并且发起模块安装。这个约束可以通过 k8s 原生的 taints 和 toleration 配合保证。

### 如何触发运维？
用户可以使用正常的 deployment 或者是其他开源社区自定义的运维 operator，只要保证其 pod template 的定义符合我们的 mock pod 的定义规范即可。

### 如何执行模块安装？
可以使用社区的 virtual-kubelet 框架，virtual-kubelet 定义了一套 kubelet 的交互生命周期，预留了一些具体的接口点，如 createPod 等。开发者通过实现这些预留的接口，便可接入 k8s 的正常的运维生命周期。

## 架构图
![](https://intranetproxy.alipay.com/skylark/lark/0/2024/jpeg/43656686/1717399526452-f12daf0a-a991-43b8-b715-157925893947.jpeg)<br />通过架构图我们不难发现，仅仅通过 virtual-kubelet 这一个简单的组件，我们就可以直接利用 k8s 实现三层调度。值得注意的事情是，POD-B 的调度会对应一个 Kube ApiServer，我们不妨称其为 ApiServerA。而触发 mock pod 调度到 mock node 上会对应另外一个 ApiServer，我们不妨称其为 ApiServerB。ApiServerA 不一定必须等于 ApiServerB，可以是彼此独立的。在一些云托管的 k8s 场景下，由于云厂商对 ApiServer 的权限限制，用户可能必须独立部署一套 ApiServer。


# 详细设计
本章节要求用户对 k8s 的运维调度体系有一定的了解，k8s 基础的运维调度流程不做复述，只对重点实现细节进行讨论。

## VPod 定义规范
在 Koupleless 体系中，模块（模块组）是一个重要的抽象，其包含如下核心属性：

- 模块名。
- 模块版本。
- 模块包地址。
- 模块运行状态等。

由于在 ModuleController V2 设计方案中，我们常使用 VPod（底层为 K8S 的 Pod）承载模块模型或模块组，因此，我们需要预先定义清楚相关模块配置到 Pod 属性的映射关系，本小结将探讨有关映射关系。<br />我们从元数据的映射开始讨论，其中，模块的元数据映射到 V1Pod 的 containers 字段下，由于 1 个 Pod 可以有 N 个 Containers，所以 V1Pod 模型天然地支持模块组的描述：

```yaml
containers:
  - name: ${MODULE_NAME}
    image: ${MODULE_URL}
    env:
      - name: MODULE_VERSION
        value: ${MODULE_VERSION}
    resource:
      requests:
        cpu: 800m
        mem: 1GI
```

相应的，模块的安装情况也可以塞在对应的 containerStatus 中，映射关系如下：

```yaml
  - containerID: arklet://{ip}:{module}:{version}
    image: {module url}
    name: moduleName
    ready: true
    started: true
    state:
      running:
        startedAt: "2024-04-25T03:53:09Z"
```

除此之外，为了方便的能通过 kubectl 通过简单的表达是把有关的 pod 筛选出来，我们还应该在 labels 里加上:<br />module.koupleless.io/${moduleName}:${version} 标签<br />模块或模块组的运行期整体状态的映射关系如下：

- 所有模块调度但未安装：pod.status.phase = 'Pending'
- 所有模块调度成功但是有几个安装失败：pod.status.phase = 'Failed'，并设置一个 condition，type 为 module.koupleless.io/installed，value 为 false。
- 所有模块调度成功并且所有都安装成功:  pod.status.phase = 'Running'，并设置一个 condition，type 为 module.koupleless.io/installed，value 为 true。

上述介绍了模块的属性的配置，除此之外，为了和 k8s 的调度和生命周期体系融合，我们还需要配置一些高阶的运行期配置。<br />我们从调度开始介绍，为了保证 VPod 只会被调度到 VNode 上，我们需要添加对应的 Affinity 配置，如下所示：

```yaml
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: basement.koupleless.io/stack
            operator: In
            values:
            - java # 多语言环境下可能有其他技术栈
          - key: basement.koupleless.io/version
            operator: In
            values:
            - ${compatiable_version}  # 模块可能只能被调度到一些特殊版本的 node 上，如有这种限制，则必须有这个字段。
```

除此之外，为了保证 VNode 只会被调度 VPod，所以 VNode 会有一些特殊的 Taints 标签，相应的，Pod 也必须添加上对应的 Tolerations，如下：

```yaml
  tolerations:
  - key: "schedule.koupleless.io/virtual-node"
    operator: "Equal"
    value: "true"
    effect: "NoExecute"
```

通过上述的 Affinity 和 Tolerations，我们可以保证 VPod 只会被调度到 VNode 上。<br />当然，我们还必须考虑这套模式和 k8s 原生流量的兼容性问题，我们可以通过 k8s 的 readinessGate 机制达到目的，添加如下配置：

```yaml
  readinessGates:
    - conditionType: "module.koupleless.io/ready" # virtual-kubelet 会根据健康检查状况跟新这个值
```

通过这些关键的规范，我们不仅能用 k8s 的 pod 模型描述模块或模块组，还能和 k8s 的调度和流量体系结合起来，一个完整的可能的样例 yaml 如下：

```yaml
apiVersion: v1
metadata:
  labels:
    module.koupleless.io/module0: 0.1.0
    module.koupleless.io/module1: 0.1.0
  name: custome-module-group-as-pod
spec:
  affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: basement.koupleless.io/stack
          operator: In
          values:
          - java # 多语言环境下可能有其他技术栈
        - key: basement.koupleless.io/version
          operator: In
          values:
          - version0
  tolerations:
  - key: "schedule.koupleless.io/virtual-node"
    operator: "Equal"
    value: "true"
    effect: "NoExecute"
  readinessGates:
  - conditionType: "module.koupleless.io/ready"
  containers:
  - name: module0
    image: http://module_url_0
    env:
      - name: MODULE_VERSION
        value: 0.1.0
    resource:
      requests:
        cpu: 800m
        mem: 1GI
  - name: module1
    image: http://module_url_1
    env:
      - name: MODULE_VERSION
        value: 0.1.0
    resource:
      requests:
        cpu: 800m
        mem: 1GI
status:
  phase: Running
  containerStatuses:
  - containerID: arklet://192.168.0.1:module0:0.1.0
    image: http://module_url_1
    name: module0
    ready: true
    started: true
    state:
      running:
        startedAt: "2024-04-25T03:53:09Z"
  - containerID: arklet://192.168.0.1:module0:0.1.0
    image: http://module_url_1
    name: module1
    ready: true
    started: true
    state:
      running:
        startedAt: "2024-04-25T03:53:09Z"
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2024-04-24T09:24:58Z"
    status: "True"
    type: basement.koupleless.io/installed
  - lastProbeTime: null
    lastTransitionTime: "2024-04-24T09:24:58Z"
    status: "True"
    type: basement.koupleless.io/ready
```


## VNode 规范设计
第一小节我们完成了 VPod 的规范设计。接下来我们需要探讨 VNode 的细则设计。<br />首先，VNode 必须有特殊的 Taints，保证正常的 Pod 不可能被调度到对应的 VNode 上，对应配置如下：

```yaml
taints:
  - effect: NoSchedule
    key: "schedule.koupleless.io/virtual-node"
    value: True
```

除此之外，我们还必须保证 VPod 只会被调度到 VNode 上。为此，Node 必须提供对应 labels，保证 Pod 能够配置相应的亲和性调度，对应配置如下：

```yaml
labels:
  basement.koupleless.io/stack: java
  basement.koupleless.io/version: ${some_version}
```

除此之外，node 还需要上报一些资源属性，如 capacity：

```yaml
capacity:
  pods: 1 # 一般来说，我们只希望一个模块被调度 1 个模块
```

以及需要定期更新 allocatable 字段：

```yaml
allocatable:
  pods: 1 
```

为了方便排障，即通过 vnode 直接找到对应的 pod，vnode 的命名规范如下：virtual-node-{stack}-{namspace}-{podname}<br />最后，vnode 的 ip 直接使用对应 pod 暴露的 ip。<br />一个可能的样例 VNode 如下:

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    basement.koupleless.io/stack: java
    basement.koupleless.io/version: version0
  creationTimestamp: "2023-07-25T13:00:00Z"
  name: virtual-node-java-example-pod-01
spec:
  taints:
    - effect: NoExecute
      key: "schedule.koupleless.io/virtual-node"
      value: True
status:
  allocatble:
    pod: 1
  capacity:
    pod: 1
```


## 自愈体系设计

VNode 需要自愈能力，原因如下。在 JVM 体系中，模块的反复安装会导致 metaspace 的使用率逐渐上涨。最终，metaspace 的使用率会达到某个阈值，过了这个阈值后模块再也无法被安装，会触发 OOM。由此，VNode 需要有一定的自愈能力，去应对这个情况。由于 Java 目前无法有效地通过 API 去完全清理干净 metaspace 中的类，因此我们将选择更简单的做法，对基座 pod 做替换，整体流程如下:

- 基座打 "schedule.koupleless.io/metaspace-overload: True: NoExecute" 的驱逐标签。
- 等待 VPod 被驱逐到别的节点。
- 执行模块卸载逻辑。
- 从基座 Pod 所对应的 ApiServer 中（有可能不是 VPod 对应的 ApiServer），删除掉基座 Pod。

如此，便可以保证 Node 的可用性。


# 实现
## 重要组件
### DaemonEndpoints
用于 kubectl 的回掉，获取 metric、日志、pod 信息等。

### nodeutil.Provider
实现 virtual-kubelet 的核心逻辑，执行具体的 pod 运维的动作如：pod 安装、pod 卸载、pod 状态获取等。

### 同步 node 节点信息
定期向 apiserver 上报 node 的信息，如 CPU、MEM 的使用量、POD 的承载量等。

## 初始化流程
virtual-kubelet 组建的初始化流程如下，按照先后顺序依次介绍。

### 初始化 K8S 证书
virtual-kubelet 和 k8s 交互依赖证书。

### 初始化 APIServer
初始化一个 golang 原生的 http 的 Mux 实例。<br />http 服务必须是加密的，因此还需要初始化对应:

- kubelet 的 ca：用于加密 kubelet 的信息。
- kubelet 的 key：用于解密服务端发送过来的信息。
- server.ca：用于加密对服务端的掉用。

这些证书不可以是任意的自签证书，具体维护逻辑可以参考：<br />[https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/)

### 初始化 Node 信息
初始化一个 virtual-node 的信息，并且上报给 ApiServer，一些关键的配置有：

- node 的 labels
- node 的 taints
- node 的 capacity
- node 的 ip 地址
- node 的 dameonEndpoint 配置

### 初始化 tracing 采点逻辑
初始化一个符合 open-tracing 踩点的逻辑。

### 启动本地的 Informer 循环
初始化 pod 和 node 的 Informer 循环。

### 初始化 Controller 循环
初始化 pod 和 node 的 controller 循环。


## NodeController 循环
主要逻辑是：

1. 初始化 node 的基础信息，并在 apiserver 创建。
2. 通过 NotifyNodeStatus 方法更新 node 的状态。
   
## PodController 循环
有 3 个核心循环：

1. 基于 k8s 的 informer 机制，不断同步服务器的 pod 信息到本地，并且更新本地的状态 / 创建 pod 实例，最终会掉用 Provider.GetPod / Provider.UpdatePod
2. 基于 k8s 的 informer 机制，不断同步服务器的待删除的（DeletionTimestamp 不为 null）pod 到本地，并在本地删除对应的 pod 实例，最终会掉用 Provider.DeletePod
3. 不断同步本地的 pod status，如果不一样则更新服务端的 pod status，会掉用 Provider.GetPodStatus 方法。
   
## Provider 实现核心

### 运行时信息映射
在 virtual-kubelet 的抽象中，vpod 是 vk 与 apiserver 交互的最小单位，模块是 vk 与 arklet 交互的最小单位。<br />在用户的视角，其提交的运维单位是 vpod，vpod 会被 vk 翻译成 n 个可能的模块，并下发给 arklet。而 vpod 和模块的对应关系，在 pod 的 spec 定义时已经通过 container 字段进行映射和描述清楚了。<br />在 vk 的视角，其需要不断的查询模块的信息，并翻译成 vpod 的状态，然后同步给 apiserver，最终更新对应的 status 字段。可是目前的问题是，模块是属于哪个 vpod的？这个信息应该记录在哪里？应该如何聚和翻译？<br />首先需要解决的问题是，如何确定模块是属于哪个 vpod 的？<br />一个可以预期的方案是，利用 containerStatus 的 containerId 字段设置成 bizIdenetity 字段，进行模块 -> vpod 的信息查找。目前 bizIdentity 的格式是 bizName:bizVersion，由系统自动生成。未来，我们希望这个 identity 可以由运维管道强制制定，格式为 vpod_{namespace}_{podName}_{moduleName}:{version}。<br />现阶段暂时使用 bizName:bizVersion 作为 bizIdenetity 的字段，不过这会带来一个问题，如果用户在 2 个 pod 上都声明了同样的 bizName+bizVersion，那么实际上在 arklet 安装的时候，会报错，因为其不支持同名同版本重复装载。不过一方面暂时用户没有这个用法，所以暂时不解决。<br />因此，vpod 到模块的关联关系，是通过 bizIdentity 这个信息去关联的。只是目前，bizIdentity 的值是 bizName:version。未来会采用 vpod_{namespace}_{podName}_{moduleName}:{version}。<br />那么接下来的问题是，如何翻译 pod 的状态？<br />pod 的状态依赖 container 的状态，而 container 的状态就是模块的状态，模块的状态可以通过 arklet 一口气查出来。我们只需要根据 bizIdentity 进行模块状态的按照关联 pod 的维度聚合，就可以获取一个完整的 contaienrStatus 状态了。其中，如果对比 vpod 的 spec 发现有模块未安装 / 安装失败，则更新为安装失败。如果都安装成功，则更新 pod 状态为安装成功。

### 从接受 vpod 到安装 / 更新 / 卸载模块
当 vk 接受到一个 vpod 后，其首先需要通过翻译模块，解析出 vpod 需要安装的模块模型，然后进行创建或者更新的调和逻辑。<br />如果是进行模块创建逻辑，则 vk 会掉用 arklet 进行有关模块的安装流程。<br />如果是进行 pod 更新的流程，则 vk 需要先对比内存中老的 pod 对应的那些模块信息，并安装新增的和删除老的。<br />如果是进行 pod 的删除流程，则 vk 需要卸载对应的模块。<br />在这里，由于分布式容错或者网络延迟等因素，jvm 层面可能出现悬挂的模块，即 vk 发现这个模块不关联到任何的模块中。因此我们需要一个兜底的守护进程，定期的去查找到这些的悬挂模块，并进行删除操作。

### Node 的信息初始化 / 状态上报 / 自愈流程
todo

