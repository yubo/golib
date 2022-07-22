# Configer

## Priority
the env variable will override default value when set flag default

```
flag > config > (env) > default
```


## env variable in config

like k8s https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/
```
version: $(VERSION)
```

or with template func
```
version: {{env "VERSION"}}
version: {{env "VERSION" | default "v1.0.0" }}
```
