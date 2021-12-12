# sensor-probe

Sensor Probe is a small utility that reads advertisement data sent by the
[Xiaomi Thermometer LYWSD03MMC](https://buy.mi.com/uk/item/3204500023) via Bluetooth LE and expose them as Prometheus metrics.

This program currently only supports sensors flashed with the [ATC_MiThermometer](https://github.com/atc1441/ATC_MiThermometer) custom firmware and the "custom" data format; if you're using the [other](https://github.com/pvvx/ATC_MiThermometer) custom firmware you
can probably just use Bluewalker to get your sensor data.

All the heavy lifting is done by [Bluewalker](https://gitlab.com/jtaimisto/bluewalker/) since I couldn't find
an easy way to read BLE events from Go and the BlueZ stack on Linux.

You should be able to find those Xiaomi sensors on [AliExpress](https://s.click.aliexpress.com/e/_AKXfnI).

## Configuration format

The configuration file is a simple TOML file:

```toml
[[sensors]]
    name = "bedroom"
    mac = "a4:c1:38:01:01:01"
    firmware = "custom"

[[sensors]]
    name = "studio"
    mac = "a4:c1:38:02:02:02"
    firmware = "custom"
```

## Usage

First you need to bring down your Bluetooth device by running `hciconfig`:

```
sudo hciconfig hci0 down
```

Then you can start `sensor-probe`.

## Credits

- The [Humidity Control with Home Assistant](https://www.splitbrain.org/blog/2021-08/16-humidity_control_with_home_assistant) blog post
on Splitbrain.org that made me discover those Xiaomi sensors.
- [atc1441](https://github.com/atc1441) for writing the [custom firmware](https://github.com/atc1441/ATC_MiThermometer) for the Xiaomi sensors.
- [Jukka Taimisto](https://gitlab.com/jtaimisto) for writing [Bluewalker](https://gitlab.com/jtaimisto/bluewalker/).
