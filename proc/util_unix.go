// +build darwin linux

package proc

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yubo/golib/orm"
	"k8s.io/klog/v2"
)

// for general startCmd
func ConfigTest(configFile string) error {
	cf, err := procInit(configFile)
	if err != nil {
		klog.Error(err)
		return err
	}

	klog.V(3).Infof("#### %s\n", configFile)
	klog.V(3).Infof("%s\n", cf)

	if err := procTest(); err != nil {
		return err
	}

	fmt.Printf("%s: configuration file %s test is successful\n",
		os.Args[0], configFile)
	return nil
}

func MainLoop(configFile string) error {
	if _, err := procInit(configFile); err != nil {
		klog.Error(err)
		panic(err)
	}

	if err := procStart(); err != nil {
		klog.Error(err)
		panic(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)

	for {
		s := <-sigs
		klog.Infof("recv %v", s)

		switch s {
		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			procStop()
			klog.Info("exit 0")
			return nil
		case syscall.SIGUSR1:
			klog.Infof("reload")
			if err := procReload(); err != nil {
				return err
			}
		default:
			klog.Infof("undefined signal %v", s)
		}
	}
}

// resetDb just for mysql
func resetDb(settings *Settings) error {
	appName := settings.Name
	configFile := settings.Config
	asset := settings.Asset
	cf, err := procInit(configFile)
	if err != nil {
		return err
	}

	driver := cf.GetStr("sys.dbDriver")
	dsn := cf.GetStr("sys.dsn")
	db, err := orm.DbOpen(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	buf, err := asset(fmt.Sprintf("%s.%s.sql", appName, driver))
	if err != nil {
		return err
	}

	fmt.Println("db", dsn)
	fmt.Println("Reset database, all datatbase tables will be drop, are you sure?")
	fmt.Print("Enter 'Yes' to continue: ")

	var in string
	fmt.Fscanf(os.Stdin, "%s", &in)
	if in != "Yes" {
		fmt.Println("cancel")
		return nil
	}

	if err = db.ExecRows(buf); err != nil {
		return err
	}
	fmt.Println("reset database successfully")
	return nil
}
