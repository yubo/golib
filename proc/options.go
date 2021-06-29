package proc

//type Options struct {
//	configFile string
//	test       bool
//}

//func (o *Options) addFlags(fs *pflag.FlagSet, name string) {
//	fs.StringVarP(&o.configFile, "config", "c", o.configFile,
//		fmt.Sprintf("config file path of your %s server.", name))
//	fs.BoolVarP(&o.test, "test", "t", o.test,
//		fmt.Sprintf("test config file path of your %s server.", name))
//}

//func newOptions(name string) *Options {
//	configFile := os.Getenv(strings.ToUpper(name) + "_CONFIG")
//	if configFile == "" {
//		configFile = fmt.Sprintf("/etc/%s/%s.yml", name, name)
//	}
//	return &Options{
//		configFile: configFile,
//		test:       false,
//	}
//}
