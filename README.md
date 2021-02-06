# telegraf-input-zpool-status

This is a simple tool to extract zpool status and output [Influx line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/);
it is designed to be used with a [telegraf exec plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/exec).
This parses the output of `zpool -H -p` and `zpool status -s -p` and has been
developed against Ubuntu 20.04 with ZFS 0.8.3 and InfluxDB 1.x for generating
compatible line protocol.

## Reference Output

This is sample `zpool -H -p` output this tool expects to parse:

```
testpool111     20401094656     18467381760     1933712896      -       -       72      90      1.00    SUSPENDED       -
testpool222     20401094656     18467233280     1933861376      -       -       68      90      1.00    DEGRADED        -
testpool333     10200547328     139776  10200407552     -       -       0       0       1.00    ONLINE  -
```

and sample `zpool status -s -p`:

```
  pool: testpool111
 state: SUSPENDED
status: One or more devices are faulted in response to IO failures.
action: Make sure the affected devices are connected, then run 'zpool clear'.
   see: http://zfsonlinux.org/msg/ZFS-8000-HC
  scan: scrub in progress since Fri Feb  5 20:00:59 2021
        17.2G scanned at 297K/s, 0B issued at 0B/s, 17.2G total
        0B repaired, 0.00% done, no estimated completion time
config:

        NAME                     STATE     READ WRITE CKSUM  SLOW
        testpool111              UNAVAIL      1     0     0     0  insufficient replicas
          /home/isaac/disk3.img  UNAVAIL      0     0     0     0  corrupted data
          /home/isaac/disk4.img  ONLINE       0     0     0     0

errors: No known data errors

  pool: testpool222
 state: DEGRADED
status: One or more devices could not be used because the label is missing or
        invalid.  Sufficient replicas exist for the pool to continue
        functioning in a degraded state.
action: Replace the device using 'zpool replace'.
   see: http://zfsonlinux.org/msg/ZFS-8000-4J
  scan: scrub repaired 0B in 0 days 00:00:27 with 0 errors on Fri Feb  5 19:54:00 2021
config:

        NAME                       STATE     READ WRITE CKSUM  SLOW
        testpool222                DEGRADED     0     0     0     0
          mirror-0                 ONLINE       0     0     0     0
            /home/isaac/disk1.img  ONLINE       0     0     0     0
            /home/isaac/disk2.img  ONLINE       0     0     0     0
          mirror-1                 DEGRADED     0     0     0     0
            /home/isaac/disk5.img  UNAVAIL      0     0    18     0  corrupted data
            /home/isaac/disk6.img  ONLINE       0     0     0     0

errors: No known data errors

  pool: testpool333
 state: ONLINE
  scan: none requested
config:

        NAME                       STATE     READ WRITE CKSUM  SLOW
        testpool333                ONLINE       0     0     0     0
          mirror-0                 ONLINE       0     0     0     0
            /home/isaac/disk7.img  ONLINE       0     0     0     0
            /home/isaac/disk8.img  ONLINE       0     0     0     0

errors: No known data errors
```

## Inreractive Run Example

The compiled tool can be run interactively. It assumes by default that the
[template](./zpool_status_template.txt) is in your current directory, but that
can be set with the `--template` CLI option.

```
./telegraf-input-zpool-status
zpool,alternative_root=-,pool=testpool111 allocated=18467381760i,capacity=90i,checkpoint=0i,dedup=1,expand=0i,fragmentation=72i,free=1933712896i,health=6i,size=20401094656i 1612641748237439221
zpool,alternative_root=-,pool=testpool222 allocated=18467233280i,capacity=90i,checkpoint=0i,dedup=1,expand=0i,fragmentation=68i,free=1933861376i,health=2i,size=20401094656i 1612641748237439221
zpool,alternative_root=-,pool=testpool333 allocated=139776i,capacity=0i,checkpoint=0i,dedup=1,expand=0i,fragmentation=0i,free=10200407552i,health=0i,size=10200547328i 1612641748237439221
zpool_device,device=testpool111,pool=testpool111 checksum_errors=0i,health=5i,notes="insufficient replicas",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk3.img,pool=testpool111 checksum_errors=0i,health=5i,notes="corrupted data",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk4.img,pool=testpool111 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_errors,pool=testpool111 errors="No known data errors",errors_found=0i 1612641748251082882
zpool_scrub,pool=testpool222 bytes_repaired=0u,errors_found=0i 1612641748251082882
zpool_device,device=testpool222,pool=testpool222 checksum_errors=0i,health=2i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=mirror-0,pool=testpool222 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk1.img,pool=testpool222 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk2.img,pool=testpool222 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=mirror-1,pool=testpool222 checksum_errors=0i,health=2i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk5.img,pool=testpool222 checksum_errors=18i,health=5i,notes="corrupted data",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk6.img,pool=testpool222 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_errors,pool=testpool222 errors="No known data errors",errors_found=0i 1612641748251082882
zpool_device,device=testpool333,pool=testpool333 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=mirror-0,pool=testpool333 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk7.img,pool=testpool333 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_device,device=/home/isaac/disk8.img,pool=testpool333 checksum_errors=0i,health=0i,notes="",read_errors=0i,slow_ios=0i,write_errors=0i 1612641748251082882
zpool_errors,pool=testpool333 errors="No known data errors",errors_found=0i 1612641748251082882
```
## Telegraf Run Example

This is a sample telegraf exec input that assumes that binary has been installed
to `/usr/local/bin/telegraf-input-zpool-status` and the [TextFSM](https://github.com/google/textfsm/wiki/TextFSM)
template to `/etc/telegraf/zpool_status_template.txt`:

```
[[inputs.exec]]                                                                 
  commands = ["/usr/local/bin/telegraf-input-zpool-status --template=/etc/telegraf/zpool_status_template.txt"]
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
pool
```

```
> show field keys from zpool_device
name: zpool_device
fieldKey        fieldType
--------        ---------
checksum_errors integer
health          integer
notes           string
read_errors     integer
slow_ios        integer
write_errors    integer
> show tag keys from zpool_device
name: zpool_device
tagKey
------
device
host
pool
```

```
> show field keys from zpool_scrub
name: zpool_scrub
fieldKey       fieldType
--------       ---------
bytes_repaired integer
errors_found   integer
> show tag keys from zpool_scrub
name: zpool_scrub
tagKey
------
host
pool
```

```
> show field keys from zpool_errors
name: zpool_errors
fieldKey     fieldType
--------     ---------
errors       string
errors_found integer
> show tag keys from zpool_errors
name: zpool_errors
tagKey
------
host
pool
```

## Health Mapping

In order to facilitate graphing I express the health as an integer. Based on the
man page I identified the following states to map:

| State | Integer |
| --- | --- |
| ONLINE | 0 |
| OFFLINE | 1 |
| DEGRADED | 2 |
| FAULTED | 3 |
| REMOVED | 4 |
| UNAVAIL | 5 |
| SUSPENDED | 6 |

The default value if a match isn't found is 99.

# Future Work

Once https://github.com/influxdata/telegraf/pull/6724 is merged the
`zpool -H -p` functionality would be redundant and a native telegraf plugin
could be used. However the `zpool status -s -p` functionality is outstanding and
could be a useful addition.

Tests should be added especially considering the sensitivity of parsing.
