package vm

import (
	"fmt"
	"github.com/dop251/goja"
	"github.com/pefish/go-error"
	"github.com/pefish/go-jsvm/pkg/vm/module"
	go_logger "github.com/pefish/go-logger"
	"github.com/pkg/errors"
	"io"
	"os"
)

type WrappedVm struct {
	Vm     *goja.Runtime
	script string
	logger go_logger.InterfaceLogger
}

type MainFuncType func([]interface{}) interface{}

func (v *WrappedVm) SetLogger(logger go_logger.InterfaceLogger) *WrappedVm {
	v.logger = logger
	return v
}

func (v *WrappedVm) Logger() go_logger.InterfaceLogger {
	return v.logger
}

func NewVm(script string) (*WrappedVm, error) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	wrappedVm := &WrappedVm{
		Vm:     vm,
		script: script,
		logger: go_logger.Logger,
	}
	err := wrappedVm.registerModules()
	if err != nil {
		return nil, err
	}
	_, err = wrappedVm.Vm.RunString(script)
	if err != nil {
		return nil, err
	}
	return wrappedVm, nil
}

func NewVmWithFile(jsFilename string) (*WrappedVm, error) {
	fileInfo, err := os.Stat(jsFilename)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	if fileInfo.IsDir() || !fileInfo.Mode().IsRegular() {
		return nil, errors.New("illegal js file")
	}
	f, err := os.Open(jsFilename)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	vm, err := NewVm(string(content))
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	return vm, nil
}

// 注册预设的一些模块
func (v *WrappedVm) registerModules() error {
	err := v.RegisterModule("console", module.NewConsoleModule(v))
	if err != nil {
		return err
	}

	err = v.RegisterModule("regex", module.NewRegexModule(v))
	if err != nil {
		return err
	}

	return nil
}

func (v *WrappedVm) RegisterModule(moduleName string, module interface{}) error {
	err := v.Vm.Set(moduleName, module)
	if err != nil {
		return go_error.WithStack(err)
	}
	return nil
}

func (v *WrappedVm) ToValue(i interface{}) goja.Value {
	return v.Vm.ToValue(i)
}

// 执行脚本中的 main 函数
func (v *WrappedVm) Run(args []interface{}) (interface{}, error) {
	return v.RunFunc("main", args)
}

func (v *WrappedVm) RunFunc(funcName string, args []interface{}) (result interface{}, err_ error) {
	defer func() {
		if err := recover(); err != nil {
			err_ = errors.Errorf("function %s run failed - %s", funcName, err.(error).Error())
		}
	}()
	if args == nil {
		args = []interface{}{"undefined"} // 必须填充一个参数，否则编译报错。goja 的问题
	}
	mainFunc, err := v.findFunc(funcName)
	if err != nil {
		return "", errors.Errorf("function %s run failed - %s", funcName, err.Error())
	}

	mainFuncResult := mainFunc(args) // panic when js throw
	return mainFuncResult, nil
}

func (v *WrappedVm) findFunc(funcName string) (result MainFuncType, err_ error) {
	defer func() {
		if err := recover(); err != nil {
			err_ = errors.Wrap(err.(error), fmt.Sprintf("js function <%s> not be found", funcName))
		}
	}()
	var mainFunc MainFuncType
	jsFunc := v.Vm.Get(funcName) // panic when not found
	err := v.Vm.ExportTo(jsFunc, &mainFunc)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("export js function <%s> error", funcName))
	}
	return mainFunc, nil
}
