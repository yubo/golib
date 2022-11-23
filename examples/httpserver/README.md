# Example

```sh
$ go run main.go
```

```sh
$ curl -XPOST http://localhost:8000 -d '
{
    "method": "Arith.String",
    "request": {
        "A": 1,
        "B": 2
    }
}'

"1+2=3"
```

```sh
$ curl -XPOST http://localhost:8000 -d '
{
    "method": "list"
}'

[
    {
        "method": "Arith.Add",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "method": "Arith.Div",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "method": "Arith.Error",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "method": "Arith.Mul",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "method": "Arith.Scan",
        "request": "",
        "response": {
            "C": 0
        }
    },
    {
        "method": "Arith.SleepMilli",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": null
    },
    {
        "method": "Arith.String",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": ""
    },
    {
        "method": "hw.Hello",
        "request": null,
        "response": ""
    }
]
```
