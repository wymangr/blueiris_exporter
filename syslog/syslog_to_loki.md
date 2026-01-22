# Setup BlueIris SysLog to Loki

In the newer versions of Blue Iris, there is a setting to "Send to SysLog server". 
As an alternative to the blueiris_exporter (or in addition to), this guide will show you how to set it up.

In this guide, I'm using NXLog as a local syslog server for Blue Iris to send the logs to. 
This forwards the logs over to Grafana Alloy which parses and ships the logs to Loki.

The provided config.alloy is configured to parse the logs and create lables similar to blueiris_exporter.
If you are just looking to get the logs into Loki and don't need the extra lables and don't
want to use the provided Grafana Dashboard, you can set your config.alloy to:

```
loki.source.syslog "blue_iris" {
  listener {
    address      = "127.0.0.1:1515"
    protocol     = "tcp"
    idle_timeout = "2h"
    labels       = { job = "blue_iris" }
  }
  forward_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "http://{{ your_loki_endpoint }}/loki/api/v1/push"
  }
}
```

### Part 1: Install NXLog Community Edition

1. Download NXLog CE:
    - Go to https://nxlog.co/products/nxlog-community-edition
    - Download the Windows MSI installer (64-bit)

1. Install NXLog:
    - Run the MSI installer
    - Use default installation path: C:\Program Files\nxlog (or C:\Program Files (x86)\nxlog)
    - Complete the installation

### Part 2: Configure NXLog to Relay Syslog

1. Edit the NXLog configuration file:

    - Open C:\Program Files\nxlog\conf\nxlog.conf in a text editor (as Administrator)
Replace the entire contents with:

        ```
        define ROOT C:\Program Files\nxlog
        define ROOT_STRING C:\Program Files\nxlog

        Moduledir %ROOT%\modules
        CacheDir %ROOT%\data
        Pidfile %ROOT%\data\nxlog.pid
        SpoolDir %ROOT%\data
        LogFile %ROOT%\data\nxlog.log

        <Extension syslog>
            Module xm_syslog
        </Extension>

        # Listen for Blue Iris syslog on UDP 1514
        <Input from_blueiris>
            Module im_udp
            Host 0.0.0.0
            Port 1514
        </Input>

        # Send to Alloy on a different port with proper RFC5424 format
        <Output to_alloy>
            Module om_tcp
            Host 127.0.0.1
            Port 1515
            Exec to_syslog_ietf();
            OutputType  LineBased
        </Output>

        <Route blueiris_to_alloy>
            Path from_blueiris => to_alloy
        </Route>
        ```

1. Start NXLog service:
    - Open Services (run services.msc)
    - Find "nxlog" service
    - Right-click → Start
    - Set it to "Automatic" startup

### Part 3: Install & Update Grafana Alloy Configuration

1. Install Alloy on Blue Iris Windows machine
    - https://grafana.com/docs/alloy/latest/set-up/install/windows/#install-grafana-alloy-on-windows

1. Update your Alloy config
    - Update `{{ your_loki_endpoint }}` in config.alloy to your loki endpoint
    - Replace config: C:\Program Files\GrafanaLabs\Alloy\config.alloy

1. Restart Alloy Service
    - Open Services (run services.msc)
    - Find "Alloy" service
    - Right-click → Start
    - Set it to "Automatic" startup

### Part 4: Configure Blue Iris

1. Open Blue Iris and click on the "Status" button in the top left
1. Check the box that says "Send to SysLog server"
1. In the server field put: `127.0.0.1:1514`
