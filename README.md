# pinger
fast pinger written on golang with http(s) api


# Main functional:

Pinger is listening on http(s) port and accepts API requests with hosts to be monitored.

Hosts can be separated by topics (i.e. "switches", "cameras", etc.) with different parameters (number of probes, update URLs). Each host also can have it's own parameters (probes, etc.) which will have higher priority then topic parameters.


### Example request generation and sending in php script:

API call should be POST request with json body, containing `topics` as `topicName` => `topicContents`.
`topicContents` should have a) parameters:
- `Probes` - number of ping requests to be sent for each host in this topic
- `Interval` - interval in seconds between pinging of each host in this topic
- `UpdateUrl` - URL, which would be requested each `updates-interval` (seconds) from config file
- `Hosts` - array of hosts to be monitored. Required parameter is `host` (ip address of monitored device). `alive` (boolean) is status of host in your DB: it needed for pinger can determine if host state is changed. Each host can also have same parameters as topic: Probes, Interval, UpdateUrl

```php
$bodyArr = [
  'switches' => [
    'Probes' => 3,
    'Interval' => 120,
    'UpdateUrl' => 'https://my-api-url/pingupdate',
    'Hosts' => [
      ['host' => '10.10.10.1', 'alive' => true],
      ['host' => '10.10.10.2', 'alive' => false],
      ['host' => '10.10.10.3', 'alive' => true]
    ]
  ],
  'cameras' => [
    'Probes' => 6,
    'Interval' => 300,
    'UpdateUrl' => 'https://my-api-url/cam-status',
    'Hosts' => [
      ['host' => '172.31.31.1', 'alive' => true],
      ['host' => '172.31.31.2', 'alive' => false]
    ]
  ]
]

$client = new \GuzzleHttp\Client();
$response = $client->post('http://pinger.local:8001/get-or-store', [RequestOptions::JSON => $bodyArr]);
```
When receiving such request, pinger compares given topics and hosts with existing ones in memory ; removing in-memory hosts that are not listed in request; adding new hosts from request.

Each `Interval` (seconds) inmemory hosts are pinged.

Each `updates-interval` (value from config) pinger send json host state updates to `UpdateUrl`.

Update is json POST request with body like `["10.10.10.1":true,"10.10.10.2":false]`


If there is `save-path` given in config file, pinger saves in-memory hosts with all parameters in file. After restart, pinger reads this file.

Hovewer, it is recommended to synchronize topics/hosts periodically with `/get-or-store` request.


If you want to add single host to some topic without full sync, you should call `/store` method instead of `/get-or-store`. In `/store`, you must pass same topics schema, but its not required to pass ALL hosts. You can pass single host and it will be added to topic. Old inmemory hosts will not be removed.


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


