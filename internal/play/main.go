package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"154.pages.dev/google/play"
	"154.pages.dev/text"
)

func main() {
	var f flags
	err := f.New()
	if err != nil {
		panic(err)
	}
	flag.BoolVar(&f.acquire, "a", false, "acquire")
	flag.StringVar(
		&play.Device.Abi, "b", play.Abi[0], strings.Join(play.Abi[1:], " "),
	)
	flag.Uint64Var(&f.app.Version, "c", 0, "version code")
	flag.BoolVar(&f.checkin, "d", false, "checkin and sync device")
	flag.StringVar(&f.app.Id, "i", "", "app ID")
	{
		var b strings.Builder
		b.WriteString("oauth_token from ")
		b.WriteString("accounts.google.com/embedded/setup/v2/android")
		flag.StringVar(&f.code, "o", "", b.String())
	}
	flag.BoolVar(&f.single, "s", false, "single APK")
	flag.BoolVar(&f.leanback, "t", false, play.Leanback)
	flag.Parse()
	text.Transport{}.Set(true)
	switch {
	case f.app.Id != "":
		switch {
		case f.acquire:
			err := f.do_acquire()
			if err != nil {
				panic(err)
			}
		case f.app.Version >= 1:
			err := f.do_delivery()
			if err != nil {
				panic(err)
			}
		default:
			details, err := f.do_details()
			if err != nil {
				panic(err)
			}
			fmt.Println(details)
		}
	case f.code != "":
		err := f.do_auth()
		if err != nil {
			panic(err)
		}
	case f.checkin:
		err := f.do_device()
		if err != nil {
			panic(err)
		}
	default:
		flag.Usage()
	}
}

func (f *flags) New() error {
	var err error
	f.home, err = os.UserHomeDir()
	if err != nil {
		return err
	}
	f.home = filepath.ToSlash(f.home) + "/google-play"
	return nil
}

type flags struct {
	acquire  bool
	app      play.StoreApp
	code     string
	checkin  bool
	home     string
	single   bool
	leanback bool
}

func (f *flags) do_delivery() error {
	checkin := &play.GoogleCheckin{}
	auth, err := f.client(checkin)
	if err != nil {
		return err
	}
	deliver, err := auth.Delivery(checkin, &f.app, f.single)
	if err != nil {
		return err
	}
	apks := deliver.Apk()
	for {
		apk, ok := apks()
		if !ok {
			break
		}
		if address, ok := apk.Url(); ok {
			if v, ok := apk.Field1(); ok {
				err := download(address, f.app.Apk(v))
				if err != nil {
					return err
				}
			}
		}
	}
	obbs := deliver.Obb()
	for {
		obb, ok := obbs()
		if !ok {
			break
		}
		if address, ok := obb.Url(); ok {
			if v, ok := obb.Field1(); ok {
				err := download(address, f.app.Obb(v))
				if err != nil {
					return err
				}
			}
		}
	}
	if v, ok := deliver.Url(); ok {
		err := download(v, f.app.Apk(""))
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *flags) do_acquire() error {
	checkin := &play.GoogleCheckin{}
	auth, err := f.client(checkin)
	if err != nil {
		return err
	}
	return auth.Acquire(checkin, f.app.Id)
}

func (f *flags) do_device() error {
	if f.leanback {
		play.Device.Feature = append(play.Device.Feature, play.Leanback)
	}
	checkin, err := play.Device.Checkin()
	if err != nil {
		return err
	}
	err = os.WriteFile(f.device_path(), checkin.Raw, 0666)
	if err != nil {
		return err
	}
	err = checkin.Unmarshal()
	if err != nil {
		return err
	}
	fmt.Println("Sleep(9*time.Second)")
	time.Sleep(9 * time.Second)
	return play.Device.Sync(checkin)
}

func (f *flags) device_path() string {
	var b strings.Builder
	b.WriteString(f.home)
	b.WriteByte('/')
	b.WriteString(play.Device.Abi)
	if f.leanback {
		b.WriteString("-leanback")
	}
	b.WriteString(".txt")
	return b.String()
}

func (f *flags) client(checkin *play.GoogleCheckin) (*play.GoogleAuth, error) {
	var (
		token play.GoogleToken
		err   error
	)
	token.Raw, err = os.ReadFile(f.home + "/token.txt")
	if err != nil {
		return nil, err
	}
	err = token.Unmarshal()
	if err != nil {
		return nil, err
	}
	auth, err := token.Auth()
	if err != nil {
		return nil, err
	}
	checkin.Raw, err = os.ReadFile(f.device_path())
	if err != nil {
		return nil, err
	}
	err = checkin.Unmarshal()
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func download(address, name string) error {
	dst, err := os.Create(name)
	if err != nil {
		return err
	}
	defer dst.Close()
	resp, err := http.Get(address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var meter text.ProgressMeter
	meter.Set(1)
	_, err = dst.ReadFrom(meter.Reader(resp))
	if err != nil {
		return err
	}
	return nil
}

func (f *flags) do_details() (*play.Details, error) {
	checkin := &play.GoogleCheckin{}
	auth, err := f.client(checkin)
	if err != nil {
		return nil, err
	}
	return auth.Details(checkin, f.app.Id, f.single)
}

func (f *flags) do_auth() error {
	var token play.GoogleToken
	err := token.New(f.code)
	if err != nil {
		return err
	}
	os.Mkdir(f.home, 0666)
	return os.WriteFile(f.home+"/token.txt", token.Raw, 0666)
}
