package v1

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	kubeprobev1 "github.com/erda-project/kubeprober/apis/v1"
)

// checker1: timeout (10)
type checker1 struct {
	Name    string
	Status  kubeprobev1.CheckerStatus
	Timeout time.Duration
}

func (c1 *checker1) GetName() string {
	return c1.Name
}

func (c1 *checker1) SetName(n string) {
	c1.Name = n
}

func (c1 *checker1) GetStatus() kubeprobev1.CheckerStatus {
	return c1.Status
}

func (c1 *checker1) SetStatus(s kubeprobev1.CheckerStatus) {
	c1.Status = s
}

func (c1 *checker1) GetTimeout() time.Duration {
	return c1.Timeout
}

func (c1 *checker1) SetTimeout(t time.Duration) {
	c1.Timeout = t
}

func (c1 *checker1) DoCheck() error {
	time.Sleep(10 * time.Second)
	return nil
}

// checker2: error
type checker2 struct {
	Name    string
	Status  kubeprobev1.CheckerStatus
	Timeout time.Duration
}

func (c2 *checker2) GetName() string {
	return c2.Name
}

func (c2 *checker2) SetName(n string) {
	c2.Name = n
}

func (c2 *checker2) GetStatus() kubeprobev1.CheckerStatus {
	return c2.Status
}

func (c2 *checker2) SetStatus(s kubeprobev1.CheckerStatus) {
	c2.Status = s
}

func (c2 *checker2) GetTimeout() time.Duration {
	return c2.Timeout
}

func (c2 *checker2) SetTimeout(t time.Duration) {
	c2.Timeout = t
}

func (c2 *checker2) DoCheck() error {
	return fmt.Errorf("mock error")
}

// checker3: info
type checker3 struct {
	Name    string
	Status  kubeprobev1.CheckerStatus
	Timeout time.Duration
}

func (c3 *checker3) GetName() string {
	return c3.Name
}

func (c3 *checker3) SetName(n string) {
	c3.Name = n
}

func (c3 *checker3) GetStatus() kubeprobev1.CheckerStatus {
	return c3.Status
}

func (c3 *checker3) SetStatus(s kubeprobev1.CheckerStatus) {
	c3.Status = s
}

func (c3 *checker3) GetTimeout() time.Duration {
	return c3.Timeout
}

func (c3 *checker3) SetTimeout(t time.Duration) {
	c3.Timeout = t
}

func (c3 *checker3) DoCheck() error {
	return nil
}

func initEnv() {
	os.Setenv("USE_MOCK", "true")
}

func TestCheckers(t *testing.T) {
	initEnv()

	c1 := checker1{
		Name:    "checker1",
		Timeout: 5 * time.Second,
	}

	c2 := checker2{
		Name: "checker2",
	}

	c3 := checker3{
		Name: "checker3",
	}

	err := RunCheckers(CheckerList{
		&c1,
		&c2,
		&c3,
	})

	assert.NoError(t, err)
}
