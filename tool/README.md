# tool
## 注意
工具默认会更改本地物理网卡的防火墙入站出站规则，以及数据包转发机制
## 配置
```
> make
```
## @虚拟网卡管理
启动网卡
```
> sudo ./up

> ifconfig
tap       Link encap:Ethernet  HWaddr 3e:80:55:c6:48:10  
          inet addr:192.168.1.1  Bcast:0.0.0.0  Mask:255.255.255.0
          UP BROADCAST MULTICAST  MTU:1500  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)
```

关闭网卡
```
> sudo ./down

```