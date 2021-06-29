简体中文 | [English](./README.md)

# KubeProber

## 什么是 KubeProber?
KubeProber 是一个针对大规模 Kubernetes 集群设计的诊断工具，用于在 kubernetes 集群中执行诊断项以证明集群的各项功能是否正常，KubeProber 有如下特点:

* **支持大规模集群** 支持多集群管理，支持在管理端配置集群跟诊断项的关系以及统一查看所有集群的诊断结果；、
* **云原生** 核心逻辑采用 [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) 来实现，提供完整的Kubernetes API兼容性;
* **可扩展** 支持用户自定义巡检项。

区别于监控系统，kubeProber 从巡检的角度来证明集群的各项功能是否正常，监控作为正向链路，无法覆盖系统的中的所有场景，系统中各个环境的监控数据都正常，也不能保证系统是 100% 可以用的，因此需要一个工具从反向来证明系统的可用性，根本上做到先于用户发现集群中不可用的点，比如：
* 集中的所有节点是否均可以被调度，有没有特殊的污点存在等；
* pod是否可以正常的创建，销毁，验证从 kubernetes，kubelet 到 docker 的整条链路；
* 创建一个service，并测试连通性，验证 kube-proxy 的链路是否正常；
* 解析一个内部或者外部的域名，验证 CoreDNS 是否正常工作；
* 访问一个 ingress 域名，验证集群中的 ingress 组件是否正常工作；
* 创建并删除一个namespace，验证相关的 webhook 是否正常工作；
* 对Etcd执行 put/get/delete 等操作，用于验证 Etcd 是否正常运行；
* 通过 mysql-client 的操作来验证 MySQL 是否正常运行；
* 模拟用户对业务系统进行登录，操作，验证业务的主流程是否常常；
* 检查各个环境的证书是否过期；
* 云资源的到期检查；
* ... 更多


## 架构
![Kubeprober Architecture](./docs/assets/architecture.jpg)

### probe-master
运行在管理集群上的 operator，这个 operator 维护两个 CRD，一个是 Cluster，用于管理被纳管的集群，另一个是 Probe，用于管理内置的以及用户自己便编写的诊断项，probe-master 通过 watch 这两个 CRD，将最新的诊断配置推送到被纳管的集群，同时 probe-master 提供接口用于查看被纳管集群的诊断结果。

### probe-agent
运行在被纳管集群上的 operator，这个 operator 维护两个 CRD，一个是跟 probe-master 完全一致的 Probe，probe-agent 按照 probe 的定义去执行该集群的诊断项，另一个是 ProbeStatus，用于记录每个 Probe 的诊断结果，用户可以在被纳管的集群中通过kubectl get probestatus 来查看本集群的诊断结果。

## 开始使用
### 安装
kubeprober的master跟agent均作为controller运行在kubernetes中，安装前确保您已经部署好了kubernetes集群，并且可以使用kubectl访问它。

#### master端的安装方法：
```
make install
APP=probe-master make deploy
```
#### agent端安装方法：

部署agent前确保您在master已经创建好了一个cluster，创建好cluster后修改configmap的配置：
```
vim config/manager-probe-agent/manager.yaml

apiVersion: v1
kind: ConfigMap
metadata:
  name: kubeprober
  namespace: system
data:
  PROBE_MASTER_ADDR: http://kubeprober-probe-master.kubeprober.svc.cluster.local:8088
  CLUSTER_NAME: moon
  SECRET_KEY: 2f5079a5-425c-4fb7-8518-562e1685c9b4
```
安装probe-agent
```
make install
APP=probe-agent make deploy
```
### 开发

你可以在本地运行以及构建probe-master以及probe-agent，运行之前请确保本地存在~/.kube/config可以访问到kubernetes集群。
#### 安装 crd && webhook
```
make dev
```
#### 运行probe-master
```
APP=probe-master make run
```
#### 运行probe-agent
```
export PROBE_MASTER_ADDR="http://127.0.0.1:8088"
export CLUSTER_NAME="moon"
export SECRET_KEY="a944499f-97f3-4986-89fa-bc7dfc7e009a" 

APP=probe-agent make run
```
#### 编译为二进制文件
```
APP=probe-master make build
APP=probe-agent make build
```
#### 构建镜像
```
APP=probe-master make docker-build
APP=probe-agent make docker-build
```
### 自定义prober
TODO
## 贡献
如果您对本项目想做出贡献，请参考 [Contributing to KubeProber](CONTRIBUTING.md) 。


## 联系
如果您有任何其他问题，欢迎跟我们取得联系。
- 邮箱: erda@terminus.io
- 知乎：[Erda技术团队](https://www.zhihu.com/people/erda-project) 
- 微信公众号:

    ![Erda WeChat](./docs/assets/wechat-small.jpg)
    
## 许可证
KubeProber 遵循 AGPL 3.0 许可证。有关详细信息请参见 [LICENSE](LICENSE) 文件。