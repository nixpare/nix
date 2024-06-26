package nix

import (
	"fmt"
	"log"

	"github.com/nixpare/logger/v3"
	"github.com/nixpare/process"
	"github.com/yookoala/gofast"
)

type PHPProcessor struct {
	connFactory gofast.ConnFactory
	Process     *process.Process
	Logger      *logger.Logger
}

func NewPHPProcessor(port int, args ...string) (php *PHPProcessor, err error) {
	php = new(PHPProcessor)

	php.Logger = logger.DefaultLogger.Clone(nil, true, "php-cgi")
	php.Process, err = process.NewProcess(
		"", "php-cgi",
		append(
			[]string{ "-b", fmt.Sprint(port) },
			process.ParseCommandArgs(args...)...
		)...)
	if err != nil {
		return
	}

	php.connFactory = gofast.SimpleConnFactory("tcp", fmt.Sprintf(":%d", port))
	return
}

func (php *PHPProcessor) Start() error {
	err := php.Process.Start(nil, nil, nil)
	if err != nil {
		return err
	}

	go func() {
		exitStatus := php.Process.Wait()
		if err := exitStatus.Error(); err != nil {
			php.Logger.Print(logger.LOG_LEVEL_ERROR, err)
		}
	}()

	return nil
}

func (php *PHPProcessor) Stop() error {
	if php.Process.IsRunning() {
		err := php.Process.Stop()
		if err != nil {
			return err
		}
	}

	exitStatus := php.Process.Wait()
	return exitStatus.Error()
}

func (ctx *Context) ServePHP(php *PHPProcessor, dir string) {
	h := gofast.NewHandler(
		gofast.NewPHPFS(dir)(gofast.BasicSession),
		gofast.SimpleClientFactory(php.connFactory),
	)

	phpLogger := ctx.Logger().Clone(nil, true, "php")

	h.SetLogger(log.New(phpLogger, "PHP", 0))
	h.ServeHTTP(ctx, ctx.R())
}
