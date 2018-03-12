# pinger
fast pinger written on golang with http(s) api


# Use cases:

## 1) Send http request and get reply instantly.
You must specify `host` and `probes` parameters in url.

Alive host example:

![](https://i.imgur.com/eTunkEm.png)

Dead host example:

![](https://i.imgur.com/T9Mkaxb.png)


## 2) Send http request and run ping job in background.
In this case after ping done, api request on configured API url will be sent:

![](https://i.imgur.com/BZDt2wO.png)

Assuming we have `"http://api.local/pingresult?host={host}&alive={alive}&rtt-ns={ns}&rtt-ms={ms}"` result-url in config, pinger will send such request:

`http://api.local/pingresult?host=10.10.10.40&alive=true&rtt-ns=297702&rtt-ms=0.297702`


## 3) Store hosts in memory and run ping jobs periodically
In this case pinger will remember given hosts and ping them with specified timeouts

Example:

![](https://i.imgur.com/sAJPCSj.png)

where:

`probes` - how many pings pinger should send during one check

`interval` - how often (in seconds) pinger will ping host (must be 30+)

### Removing host from pool:

![](https://i.imgur.com/gxIJtmX.png)

