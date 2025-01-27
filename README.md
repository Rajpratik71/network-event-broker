### network-event-broker
----
A daemon configures network and executes scripts on network events such as `systemd-networkd's` [DBus](https://www.freedesktop.org/wiki/Software/dbus/) events,
`dhclient` gains lease lease. It also watches when

1. An address getting added/removed/modified.
2. Links added/removed.

```network-event-broker``` creates link state directories ```carrier.d```,  ```configured.d```,  ```degraded.d```  ```no-carrier.d```  ```routable.d``` and manager state dir ```manager.d``` in ```/etc/network-event-broker```. Executable scripts can be placed into directories.

Use cases:

How to run a command when get a new address is acquired via DHCP ?

1. `systemd-networkd's`
 Scripts are executed when the daemon receives the relevant event from `systemd-networkd`. See [networkctl](https://www.freedesktop.org/software/systemd/man/networkctl.html).


```bash
May 14 17:08:13 Zeus cat[273185]: OperationalState="routable"
May 14 17:08:13 Zeus cat[273185]: LINK=ens33
```

2. `dhclient`
  For `dhclient` scripts will be executed (in the dir ```routable.d```) when the `/var/lib/dhclient/dhclient.leases` file gets modified by `dhclient` and lease information is passed to the scripts as environmental arguments.

Environment variables `LINK`, `LINKINDEX=` and DHCP lease information `DHCP_LEASE=`  passed to the scripts.

#### How can I make my secondary network interface work ?

 When both interfaces are in same subnet and we have only one routing table with one GW, ie. traffic that reach via eth2 tries to leave via eth0(primary interface) which it can't. So we need to add a secondary routing table and routing policy so that the secondary interface uses the new custom routing table. Incase of static address the address and the routes already know. Incase of DHCP it's not prodictable. `network-event-broker` automatically configures the routing policy rules via ```RoutingPolicyRules=```. 

#### Building from source
----

```bash

❯ make build
❯ sudo make install

```

Due to security `network-broker` runs in non root user `network-broker`. It drops all privileges except CAP_NET_ADMIN and CAP_SYS_ADMIN.

```bash
❯  useradd -M -s /usr/bin/nologin network-broker
```

### Configuration
----

Configuration file `network-broker.toml` located in ```/etc/network-broker/``` directory to manage the configuration.

The `[System]` section takes following Keys:
``` bash

LogLevel=
```
Specifies the log level. Takes one of `info`, `warn`, `error`, `debug` and `fatal`. Defaults to `info`.

```bash

Generator= 
```
Specifies the network event generator source to listen. Takes one of `systemd-networkd` or `dhclient`. Defaults to `systemd-networkd`.


The `[Network]` section takes following Keys:

```bash

Links=
```
A whitespace-separated list of links whose events should be monitored. Defaults to unset.

```bash

RoutingPolicyRules=
```
A whitespace-separated list of links for which routing policy rules would be configured per address. When set, `network-broker` automatically adds routing policy rules `from` and `to` in another routing table `(ROUTE_TABLE_BASE = 9999 + ifindex)`. When these addresses are removed, the routing policy rules are also dropped. Defaults to unset.

```bash
EmitJSON=
```
A boolean. When true, JSON format data will be emitted via envorment variable `JSON=` Applies only for `systemd-networkd`. Defaults to true.

```json
{
   "AddressState":"routable",
   "AlternativeNames":null,
   "CarrierState":"carrier",
   "Driver":"e1000",
   "IPv4AddressState":"routable",
   "IPv6AddressState":"degraded",
   "Index":2,
   "LinkFile":"",
   "Model":"82545EM Gigabit Ethernet Controller (Copper)",
   "Name":"ens33",
   "OnlineState":"online",
   "OperationalState":"routable",
   "Path":"pci-0000:02:01.0",
   "SetupState":"configured",
   "Type":"ether",
   "Vendor":"Intel Corporation",
   "Manufacturer":"",
   "NetworkFile":"/etc/systemd/network/10-ens33.network",
   "DNS":[
      "172.16.130.2"
   ],
   "Domains":null,
   "NTP":null
}

```

```bash
UseDNS=
```
A boolean. When true, the DNS server will be se to `systemd-resolved` vis DBus. Applies only for DHClient. Defaults to false.

```bash
UseDomain=
```
A boolean. When true, the DNS domains will be sent to `systemd-resolved` vis DBus. Applies only for DHClient. Defaults to false.

```bash
UseHostname=
```
A boolean. When true, the host name be sent to `systemd-hostnamed` vis DBus. Applies only for DHClient. Defaults to false.

```bash
❯ sudo cat /etc/network-broker/network-broker.toml 
[System]
LogLevel="debug"
Generator="systemd-networkd"

[Network]
Links="eth0 eth1"
RoutingPolicyRules="eth0 eth1"
UseDNS="true"
UseDomain="true"
EmitJSON="true"

```

```bash

❯ systemctl status network-broker.service
● network-broker.service - A daemon configures network upon events
     Loaded: loaded (/usr/lib/systemd/system/network-broker.service; disabled; vendor preset: disabled)
     Active: active (running) since Thu 2021-06-03 22:22:38 CEST; 3h 13min ago
       Docs: man:networkd-broker.conf(5)
   Main PID: 572392 (network-broker)
      Tasks: 7 (limit: 9287)
     Memory: 6.2M
        CPU: 319ms
     CGroup: /system.slice/network-broker.service
             └─572392 /usr/bin/network-broker

Jun 04 01:36:04 Zeus network-broker[572392]: [info] 2021/06/04 01:36:04 Link='ens33' ifindex='2' changed state 'OperationalState'="carrier"
Jun 04 01:36:04 Zeus network-broker[572392]: [info] 2021/06/04 01:36:04 Link='' ifindex='1' changed state 'OperationalState'="carrier"

```
DBus signals generated by ```systemd-networkd```
```bash

&{:1.683 /org/freedesktop/network1/link/_32 org.freedesktop.DBus.Properties.PropertiesChanged [org.freedesktop.network1.Link map[AdministrativeState:"configured"] []] 10}
```

```
‣ Type=signal  Endian=l  Flags=1  Version=1 Cookie=24  Timestamp="Sun 2021-05-16 08:06:05.905781 UTC"
  Sender=:1.292  Path=/org/freedesktop/network1  Interface=org.freedesktop.DBus.Properties  Member=PropertiesChanged
  UniqueName=:1.292
  MESSAGE "sa{sv}as" {
          STRING "org.freedesktop.network1.Manager";
          ARRAY "{sv}" {
                  DICT_ENTRY "sv" {
                          STRING "OperationalState";
                          VARIANT "s" {
                                  STRING "degraded";
                          };
                  };
          };
          ARRAY "s" {
          };
  };

```


#### Contributing
----

The **Network Event Broker** project team welcomes contributions from the community. If you wish to contribute code and you have not signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq).

slack channel [#photon](https://code.vmware.com/web/code/join).

#### License
----

[Apache-2.0](https://spdx.org/licenses/Apache-2.0.html)
