# ORM

- Little Featured ORM
- Auto Migrations

```shell
$ go run ./main.go
I0110 17:23:44.965517   53915 base.go:96] SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=`user`
I0110 17:23:44.965711   53915 base.go:34] CREATE TABLE `user` (`name` text,`age` integer,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`name`))
I0110 17:23:44.965927   53915 base.go:123] INSERT INTO `user` (`name`, `age`, `created_at`, `updated_at`) VALUES (`tom`, `0`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`, `2022-01-10 17:23:44.965903 +0800 CST m=+0.002683397`)
I0110 17:23:44.965983   53915 base.go:198] UPDATE `user` SET `age` = `17`, `updated_at` = `2022-01-10 17:23:44.965975 +0800 CST m=+0.002756070` WHERE `name` = `tom`
I0110 17:23:44.966075   53915 base.go:183] SELECT * FROM `user` WHERE `name` = `tom`
get user {Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}
I0110 17:23:44.966215   53915 base.go:157] SELECT * FROM `user`
I0110 17:23:44.966271   53915 base.go:163] SELECT COUNT(*) FROM `user`
get users: [1] [{Name:tom Age:17 CreatedAt:2022-01-10 17:23:44.965903 +0800 +0800 UpdatedAt:2022-01-10 17:23:44.965975 +0800 +0800}]
I0110 17:23:44.966341   53915 base.go:213] DELETE FROM `user` WHERE `name` = `tom`
```

## References
- https://github.com/yubo/golib/blob/main/examples/orm/golib-orm.go
- https://github.com/yubo/apiserver/tree/main/examples/rest
