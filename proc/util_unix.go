// +build darwin linux

package proc

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
)

// for general startCmd
func (p *Module) testConfig() error {
	configFile := p.config
	cf, err := p.procInit(configFile)
	if err != nil {
		klog.Error(err)
		return err
	}

	klog.V(3).Infof("#### %s\n", configFile)
	klog.V(3).Infof("%s\n", cf)

	if err := p.procTest(); err != nil {
		return err
	}

	fmt.Printf("%s: configuration file %s test is successful\n",
		os.Args[0], configFile)
	return nil
}

func (p *Module) start() error {
	if LogBuildInfoAtStartup != "" {
		LogBuildInfo()
	}

	if _, err := p.procInit(p.config); err != nil {
		klog.Error(err)
		panic(err)
	}

	if err := p.procStart(); err != nil {
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
			p.procStop()
			klog.Info("exit 0")
			return nil
		case syscall.SIGUSR1:
			klog.Infof("reload")
			if err := p.procReload(); err != nil {
				return err
			}
		default:
			klog.Infof("undefined signal %v", s)
		}
	}
}
