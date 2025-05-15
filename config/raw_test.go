package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_VariableInterpolation_1_detectsRegex(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	t.Setenv("MY_var", "SUCCESS")

	matchingExpression := "__${MY_var}__"
	sut := interpolate(matchingExpression)

	assertion.True(sut == "SUCCESS")
}

func Test_VariableInterpolation_2_detectsRegex(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	t.Setenv("MY_2nd_var", "SUCCESS")

	matchingExpression := "__${MY_2nd_var}__"
	sut := interpolate(matchingExpression)

	assertion.True(sut == "SUCCESS")
}

func Test_VariableInterpolation_3_failing(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	failingExpression := "_-${MY_failing_expression}__"
	sut := interpolate(failingExpression)

	assertion.False(sut == "MY_failing_expression")
	assertion.True(sut == failingExpression)
}

func Test_VariableInterpolation_4_detectsRegexpWithNumbers(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	t.Setenv("MY_var_with_1234_NUMBERS", "SUCCESS")

	matchingExpression := "__${MY_var_with_1234_NUMBERS}__"
	sut := interpolate(matchingExpression)

	assertion.True(sut == "SUCCESS")
}

func Test_VariableInterpolation_4_failsRegexpWithDashes(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	failingExpression := "__${MY-var-with-DASHES}__"
	sut := interpolate(failingExpression)

	assertion.False(sut == "MY-var-with-DASHES")
	assertion.True(sut == failingExpression)
}

func Test_VariableInterpolation_5_EnvVariableMissing(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	matchingExpression := "__${my_missing_var}__"
	sut := interpolate(matchingExpression)

	assertion.True(sut == "")
}

func Test_VariableInterpolation_5_interpolatesTemplateString(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertion := assert.New(t)

	t.Setenv("S3_HOST", "s3.my-company.com")
	t.Setenv("S3_PORT", "1234")

	matchingExpression := "__${S3_HOST}__:__${S3_PORT}__"
	sut := interpolate(matchingExpression)

	assertion.True(sut == "s3.my-company.com:1234")
}
