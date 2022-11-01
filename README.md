# blueiris_exporter
Prometheus exporter for Blue Iris.
Confirmed working on Blue Iris version 5.6.2.8

## Flags

Flag     | Description | Default value | Required
---------|-------------|--------------------|---
`--cameras` | Comma-separated list of camera shot names that match Blue Iris | None | Yes
`--telemetry.addr` | addresses on which to expose metrics | `:2112` | No
`--logpath` | Directory path to the Blue Iris Logs | `C:\BlueIris\log\` | Yes
`--telemetry.path` | URL path for surfacing collected metrics" | `/metrics` | No

## Installation and Usage
`blueiris_exporter` listens on HTTP port 2112 by default. See the `--help` output for more options.

You need to make sure that Blue Iris is saving the log to a file. 
1. Click the Status Button at the top left of Blue Iris
2. Select the `Log` tab
3. Check the box `Save to file`

By Default, Blue Iris will break out your log files by month. This means the counter metrics will reset at the beginning of each month. If you don't want this to happen, concider changing the name of your log files. 


## Windows

Install service

### RHEL/CentOS/Fedora

Download the latest release and setup as a service

### Docker

The Docker image is not hosted yet on Docker Hub, so you will need to build the image.

```
docker build -t <image_name>:<tag> .
```
You can then start up the container, passing in the Blue Iris log directory and a list of camera short names.
```bash
docker run -d \
  -p 2112:2112 \
  -v "/path/to/blueiris/log:/path/to/blueiris/log" \
  <image_name>:<tag> \
  --logpath=/path/to/blueiris/log --cameras=FD,BY,DW
```

For Docker compose, see example below.

```yaml
---
version: '3.8'
services:

  blueiris_exporter:
    restart: unless-stopped
    build:
      context: .
      dockerfile: Dockerfile
    command: ["--logpath=/path/to/blueiris/log", "--cameras=FD,BY,DW"]
    ports:
      - 2112:2112
    volumes:
      - /path/to/blueiris/log:/path/to/blueiris/log

```

## Metrics

Name     | Description |
---------|-------------|
ai_duration | Duration (ms) of the last Blue Iris alert for each camera. This metric will continue to expose the last duration each time it's scraped
ai_duration_distinct | Duration (ms) of the last Blue Iris alert for each camera. This metric will only show new alerts and will disapear the next scrpe
ai_count | Count of the number of times IA analyzed and image
ai_restarted | Number of times Blue Iris restarted the AI (deepstack)
logerror | Count of unique errors in the logs
