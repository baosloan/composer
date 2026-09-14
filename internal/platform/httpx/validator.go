package httpx

import (
	"reflect"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// SetupValidator 用于配置 Gin 在参数绑定时使用的校验器实例。
//
// 应在应用启动阶段、开始提供服务之前调用一次。这里主要完成两件事：
//
//  1. 校验错误消息中的字段名取自 json 标签，
//     因此客户端看到的是“password 长度至少为 8 个字符”，
//     而不是“Password ...”。
//  2. 项目级的自定义校验规则会在这里统一注册，
//     这样各个模块只需要写 binding:"strong_password" 即可使用对应规则。
func SetupValidator() error {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		// Gin 更换了其校验引擎；参数绑定本身仍然可以正常工作，
		// 只是无法再使用更友好的字段名和自定义校验规则。
		return nil
	}

	v.RegisterTagNameFunc(jsonFieldName)

	rules := map[string]validator.Func{
		"strong_password": strongPassword,
		"no_space":        noSpace,
		"slug":            slug,
	}
	for tag, fn := range rules {
		if err := v.RegisterValidation(tag, fn); err != nil {
			return err
		}
	}
	return nil
}

// jsonFieldName 将结构体字段解析为客户端实际发送的字段名。
func jsonFieldName(f reflect.StructField) string {
	name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
	if name == "" {
		name = strings.SplitN(f.Tag.Get("form"), ",", 2)[0]
	}
	if name == "-" || name == "" {
		return f.Name
	}
	return name
}

// strongPassword 要求密码必须同时包含大写字母、小写字母和数字。
// 密码长度则交由 min/max 规则校验，以便不同字段可以设置各自的长度限制。
func strongPassword(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	var hasUpper, hasLower, hasDigit bool
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// noSpace 用于拒绝任何包含空白字符的内容， 适用于用户名等标识符字段。
func noSpace(fl validator.FieldLevel) bool {
	return !strings.ContainsFunc(fl.Field().String(), unicode.IsSpace)
}

// slug 仅允许包含小写字母、数字、连字符和下划线。
func slug(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
