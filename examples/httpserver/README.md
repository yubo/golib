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
        "serviceMethod": "Arith.Add",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "serviceMethod": "Arith.Div",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "serviceMethod": "Arith.Error",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "serviceMethod": "Arith.Mul",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": {
            "C": 0
        }
    },
    {
        "serviceMethod": "Arith.Scan",
        "request": "",
        "response": {
            "C": 0
        }
    },
    {
        "serviceMethod": "Arith.SleepMilli",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": null
    },
    {
        "serviceMethod": "Arith.String",
        "request": {
            "A": 0,
            "B": 0
        },
        "response": ""
    },
    {
        "serviceMethod": "hw.Hello",
        "request": null,
        "response": ""
    }
]
```
