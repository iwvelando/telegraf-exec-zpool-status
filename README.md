# telegraf-input-zpool-status

This is a simple tool to extract zpool status and output [Influx line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/);
it is designed to be used with a [telegraf exec plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/exec).
This parses the output of `zpool -H -p` and has been developed against
Ubuntu 20.04 with ZFS 0.8.3 and InfluxDB 1.x for generating compatible line
protocol.

## Interactive Run Example

The compiled tool can be run interactively:

```
./telegraf-input-zpool-status
zpool,alternative_root=-,name=testpool allocated=127488i,capacity=0i,checkpoint=0i,dedup=1,expand=0i,fragmentation=0i,free=204009671
68i,health=3i,size=20401094656i 1612490775507703812
zpool,alternative_root=-,name=testpool111 allocated=122880i,capacity=0i,checkpoint=0i,dedup=1,expand=0i,fragmentation=0i,free=204009
71776i,health=3i,size=20401094656i 1612490775507703812
```
## Telegraf Run Example

Add this input to a telegraf configuration (adapting to installed binary path):

```
[[inputs.exec]]
  commands = ["/usr/local/bin/telegraf-input-zpool-status"]
  timeout = "5s"
  data_format = "influx"
```

Then in InfluxDB:

```
> show field keys from zpool
name: zpool
fieldKey      fieldType
--------      ---------
allocated     integer
capacity      integer
checkpoint    integer
dedup         float
expand        integer
fragmentation integer
free          integer
health        integer
size          integer
> show tag keys from zpool
name: zpool
tagKey
------
alternative_root
host
name
```

## Health Mapping

In order to facilitate graphing I express the healt as an integer. Based on the
man page I identified the following states to map:

| State | Integer |
| --- | --- |
| DEGRADED | 0 |
| FAULTED | 1 |
| OFFLINE | 2 |
| ONLINE | 3 |
| REMOVED | 4 |
| UNAVAIL | 5 |

The default value if a match isn't found is -1.

# Future Work

Once https://github.com/influxdata/telegraf/pull/6724 is merged the
`zpool -H -p` functionality would be redundant and a native telegraf plugin
could be used.

For zpool health monitoring I also want to monitor the read, write, and checksum
errors reported by `zpool status`; that functionality is not currently planned
in the aforementioned pull request so that would remain useful here.
