package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	appPkg "github.com/mattfan00/jvbe/app"
	"github.com/mattfan00/jvbe/auth"
	"github.com/mattfan00/jvbe/config"
	"github.com/mattfan00/jvbe/db"
	"github.com/mattfan00/jvbe/event"
	"github.com/mattfan00/jvbe/group"
	"github.com/mattfan00/jvbe/template"
	"github.com/mattfan00/jvbe/user"
	"gopkg.in/mail.v2"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
)

type appProgram struct {
	fs         *flag.FlagSet
	args       []string
	configPath string
}

func newAppProgram(args []string) *appProgram {
	fs := flag.NewFlagSet("app", flag.ExitOnError)
	p := &appProgram{
		fs:   fs,
		args: args,
	}

	fs.StringVar(&p.configPath, "c", "./config.yaml", "path to config file")

	return p
}

func (p *appProgram) parse() error {
	return p.fs.Parse(p.args)
}

func (p *appProgram) name() string {
	return p.fs.Name()
}

func (p *appProgram) run() error {
	conf, err := config.ReadFile(p.configPath)
	if err != nil {
		return err
	}

	db, err := db.Connect(conf.DbConn)
	if err != nil {
		return err
	}

	templates, err := template.Generate()
	if err != nil {
		return err
	}

	gob.Register(user.SessionUser{}) // needed for scs library
	session := scs.New()
	session.Lifetime = 30 * 24 * time.Hour // 30 days
	session.Store = sqlite3store.New(db.DB.DB)

	smtp := mail.NewDialer(conf.EmailServer, 587, conf.EmailSender, conf.EmailPass)
	groupService := group.NewService(db)
	eventService := event.NewService(db, smtp)
	userService := user.NewService(db)

	authService, err := auth.NewService(conf)
	if err != nil {
		return err
	}

	app := appPkg.New(
		eventService,
		userService,
		authService,
		groupService,

		conf,
		session,
		templates,
	)

	log.Printf("listening on port %d", conf.Port)
	http.ListenAndServe(fmt.Sprintf(":%d", conf.Port), app.Routes())

	return nil
}
