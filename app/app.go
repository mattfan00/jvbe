package app

import (
	"net/http"

	"github.com/mattfan00/jvbe/app/template"
	"github.com/mattfan00/jvbe/auditlog"
	"github.com/mattfan00/jvbe/auth"
	"github.com/mattfan00/jvbe/config"
	"github.com/mattfan00/jvbe/event"
	"github.com/mattfan00/jvbe/group"
	"github.com/mattfan00/jvbe/logger"
	"github.com/mattfan00/jvbe/user"

	"github.com/alexedwards/scs/v2"
	"github.com/gorilla/schema"
)

type App struct {
	eventService    event.Service
	userService     user.Service
	authService     auth.Service
	groupService    group.Service
	auditlogService auditlog.Service

	conf            *config.Config
	session         *scs.SessionManager
	log             logger.Logger
	templateManager *template.Manager
}

func New(
	eventService event.Service,
	userService user.Service,
	authService auth.Service,
	groupService group.Service,
	auditlogService auditlog.Service,

	conf *config.Config,
	session *scs.SessionManager,
	log logger.Logger,
) *App {
	templateManager := template.NewManager(log)

	return &App{
		eventService:    eventService,
		userService:     userService,
		authService:     authService,
		groupService:    groupService,
		auditlogService: auditlogService,

		conf:            conf,
		session:         session,
		log:             log,
		templateManager: templateManager,
	}
}

type BaseData struct {
	User user.SessionUser
}

func (a *App) renderPage(w http.ResponseWriter, pageFile string, data any) {
	files := []string{"base.html", "header.html"}
	files = append(files, pageFile)
	t, err := a.templateManager.Parse(pageFile, files)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, data)
}

func (a *App) renderTemplate(w http.ResponseWriter, templateFile string, data any) {
	t, err := a.templateManager.Parse(templateFile, []string{templateFile})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, data)
}

func (a *App) renderErrorNotif(
	w http.ResponseWriter,
	err error,
	status int,
) {
	a.log.Errorf(err.Error())
	w.Header().Add("HX-Reswap", "none") // so that UI does not swap rest of the blank template
	w.WriteHeader(status)
	a.renderTemplate(w, "error-notif.html", map[string]any{
		"Error": err,
	})
}

func (a *App) renderErrorPage(
	w http.ResponseWriter,
	err error,
	status int,
) {
	a.log.Errorf(err.Error())
	w.Header().Add("HX-Retarget", "body")
	w.Header().Add("HX-Reswap", "innerHTML")
	w.WriteHeader(status)
	a.renderPage(w, "error.html", map[string]any{
		"Error": err,
	})
}

func (a *App) renewSessionUser(r *http.Request, u *user.SessionUser) error {
	err := a.session.RenewToken(r.Context())
	if err != nil {
		return err
	}

	a.session.Put(r.Context(), "user", u)

	return nil
}

func schemaDecode[T any](r *http.Request) (T, error) {
	var v T

	if err := r.ParseForm(); err != nil {
		return v, err
	}

	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	if err := decoder.Decode(&v, r.PostForm); err != nil {
		return v, err
	}

	return v, nil
}
