## Config example

you can set config by (descending order by priority)

- [Command Line Arguments](#command-line-arguments)
- [YAML file](#yaml-file)
- [Environment Variables](#environment-variables)

#### Command Line Arguments

```
$ go run ./golib-config.go -h


Usage:
  golibConfig [flags]

Golib examples flags:

      --city string
                city (default "beijing")
      --phone string
                phone number
      --user-age int
                user age
      --user-name string
                user name (default "yubo")

Global flags:
...
```

default setting
```
$ go run ./golib-config.go
city: beijing
phone: ""
userAge: 0
userName: yubo
I1104 14:22:08.374445   22053 proc.go:275] See ya!
```

- use --city
```
$ go run ./golib-config.go ./golib-config.go --city NewYork --user-name tom
city: NewYork
phone: ""
userAge: 0
userName: tom
I1104 14:24:35.979256   22733 proc.go:275] See ya!
```

- use --set-string as yaml.path
```
$go run ./golib-config.go  --set-string=golibConfig.city=wuhan --set-string=golibConfig.userName=yubo
city: wuhan
phone: ""
userAge: 0
userName: yubo
I1104 15:11:54.053312   54259 proc.go:275] See ya!
```

#### YAML file

- [config.yaml](./config.yaml)

```yaml
golibConfig:
  userName: bajie
  userAge: 16
  city: gao
  phone: 14159265359
```

```shell
$ go run ./golib-config.go -f ./config.yaml
city: gao
phone: "14159265359"
userAge: 16
userName: bajie
I1104 14:18:54.172979   21185 proc.go:275] See ya!
```

#### Environment Variables
```
$ USER_NAME=wukong USER_CITY="Flowers and Fruit Mountain" go run ./golib-config.go
city: Flowers and Fruit Mountain
phone: ""
userAge: 0
userName: wukong
I1104 15:30:33.678388   60856 proc.go:275] See ya!
```
