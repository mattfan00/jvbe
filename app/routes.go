package app

import (
	"fmt"
	eventPkg "github/mattfan00/jvbe/event"
	groupPkg "github/mattfan00/jvbe/group"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()

	publicFileServer := http.FileServer(http.Dir("./ui/public"))
	r.Handle("/public/*", http.StripPrefix("/public/", publicFileServer))

	r.Get("/privacy", a.renderPrivacy)

	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitAll(100, 1*time.Minute))
		r.Use(middleware.Logger)
		r.Use(a.recoverPanic)
		r.Use(a.session.LoadAndSave)

		r.Get("/", a.renderIndex)

		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", a.renderLogin)
			r.Get("/callback", a.handleLoginCallback)

			r.With(a.requireAuth).Get("/logout", a.handleLogout)
		})

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)

			r.Get("/home", a.renderHome)

			r.Route("/event", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(a.canModifyEvent)

					r.Get("/new", a.renderNewEvent)
					r.Post("/new", a.createEvent)
					r.Get("/{id}/edit", a.renderEditEvent)
					r.Post("/{id}/edit", a.updateEvent)
					r.Delete("/{id}/edit", a.deleteEvent)
				})

				r.Get("/{id}", a.renderEventDetails)
				r.Post("/respond", a.respondEvent)
			})

			r.Route("/group", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(a.canModifyGroup)

					r.Get("/list", a.renderGroupList)
					r.Get("/new", a.renderNewGroup)
					r.Post("/new", a.createGroup)
					r.Get("/{id}/edit", a.renderEditGroup)
					r.Post("/{id}/edit", a.updateGroup)
					r.Delete("/{id}/edit", a.deleteGroup)
					r.Delete("/{id}/member/{userId}", a.removeGroupMember)
					r.Post("/{id}/invite", a.refreshInviteLinkGroup)
				})

				r.Get("/{id}/invite", a.inviteGroup)
				r.Get("/{id}", a.renderGroupDetails)
			})
		})
	})

	return r
}

func (a *App) renderPrivacy(w http.ResponseWriter, r *http.Request) {
	a.renderPage(w, "privacy.html", nil)
}

func (a *App) renderIndex(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.sessionUser(r); ok {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}

	a.renderPage(w, "index.html", nil)
}

type homeData struct {
	BaseData
	CurrEvents []eventPkg.Event
}

func (a *App) renderHome(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	currEvents, err := a.event.ListCurrent(u.Id)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "home.html", homeData{
		BaseData: BaseData{
			User: u,
		},
		CurrEvents: currEvents,
	})
}

type newEventData struct {
	BaseData
	Groups []groupPkg.Group
}

func (a *App) renderNewEvent(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	g, err := a.group.List()
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "event/new.html", newEventData{
		BaseData: BaseData{
			User: u,
		},
		Groups: g,
	})
}

func (a *App) createEvent(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	req, err := schemaDecode[eventPkg.CreateRequest](r)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}
	req.CreatorId = u.Id

	err = a.event.Create(req)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

type eventDetailsData struct {
	BaseData
	Event            eventPkg.EventDetailed
	MaxAttendeeCount int
}

func (a *App) renderEventDetails(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)
	eventId := chi.URLParam(r, "id")

	e, err := a.event.GetDetailed(eventId, u.Id)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "event/details.html", eventDetailsData{
		BaseData: BaseData{
			User: u,
		},
		Event:            e,
		MaxAttendeeCount: eventPkg.MaxAttendeeCount,
	})
}

func (a *App) respondEvent(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	req, err := schemaDecode[eventPkg.RespondEventRequest](r)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}
	req.UserId = u.Id

	err = a.event.HandleResponse(req)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/event/"+req.Id, http.StatusSeeOther)
}

func (a *App) deleteEvent(w http.ResponseWriter, r *http.Request) {
	eventId := chi.URLParam(r, "id")

	err := a.event.Delete(eventId)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

type editEventData struct {
	BaseData
	Event eventPkg.Event
}

func (a *App) renderEditEvent(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)
	id := chi.URLParam(r, "id")

	e, err := a.event.Get(id)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "event/edit.html", editEventData{
		BaseData: BaseData{
			User: u,
		},
		Event: e,
	})
}

// TODO
func (a *App) updateEvent(w http.ResponseWriter, r *http.Request) {
	/*
		u, _ := a.sessionUser(r)
		id := chi.URLParam(r, "id")
		log.Printf("user updating event %s: %s", id, u.Id)

		if err := r.ParseForm(); err != nil {
			a.renderErrorNotif(w, err, http.StatusInternalServerError)
			return
		}

		var req eventPkg.UpdateRequest
		if err := schema.NewDecoder().Decode(&req, r.PostForm); err != nil {
			a.renderErrorNotif(w, err, http.StatusInternalServerError)
			return
		}
		req.Id = id

		if err := a.event.Update(req); err != nil {
			a.renderErrorNotif(w, err, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/event/"+id, http.StatusSeeOther)
	*/
	w.Write(nil)
}

func (a *App) renderLogin(w http.ResponseWriter, r *http.Request) {
	state, err := gonanoid.New()
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
	}

	a.session.Put(r.Context(), "state", state)

	url := a.auth.AuthCodeUrl(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (a *App) handleLoginCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("login callback: %s", r.URL.String())

	state := r.URL.Query().Get("state")
	expectedState := a.session.PopString(r.Context(), "state")
	if state != expectedState {
		err := fmt.Errorf("invalid oauth state, expected '%s', got '%s'", expectedState, state)
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")
	u, err := a.auth.HandleLogin(code)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	err = a.session.RenewToken(r.Context())
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.session.Put(r.Context(), "user", &u)

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	err := a.session.Destroy(r.Context())
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	redirect := fmt.Sprintf("https://%s/logout?redirect=%s", a.conf.Oauth.Domain, a.conf.OauthLogoutRedirectUrl())
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (a *App) renderNewGroup(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	a.renderPage(w, "group/new.html", BaseData{
		User: u,
	})
}

func (a *App) createGroup(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	req, err := schemaDecode[groupPkg.CreateRequest](r)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}
	req.CreatorId = u.Id

	err = a.group.CreateAndAddMember(req)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/group/list", http.StatusSeeOther)
}

type groupDetailsData struct {
	BaseData
	Group groupPkg.GroupDetailed
}

func (a *App) renderGroupDetails(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)
	id := chi.URLParam(r, "id")

	g, err := a.group.GetDetailed(id, u)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "group/details.html", groupDetailsData{
		BaseData: BaseData{
			User: u,
		},
		Group: g,
	})
}

type groupListData struct {
	BaseData
	Groups []groupPkg.Group
}

func (a *App) renderGroupList(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	g, err := a.group.List()
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "group/list.html", groupListData{
		BaseData: BaseData{
			User: u,
		},
		Groups: g,
	})
}

func (a *App) inviteGroup(w http.ResponseWriter, r *http.Request) {
	u, ok := a.sessionUser(r)
	if !ok {
		w.Write([]byte("need to redirect login here"))
		return
	}

	id := chi.URLParam(r, "id")

	g, err := a.group.AddMemberFromInvite(id, u.Id)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "group/invite.html", g)
}

type editGroupData struct {
	BaseData
	Group groupPkg.Group
}

func (a *App) renderEditGroup(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)
	id := chi.URLParam(r, "id")

	g, err := a.group.Get(id)
	if err != nil {
		a.renderErrorPage(w, err, http.StatusInternalServerError)
		return
	}

	a.renderPage(w, "group/edit.html", editGroupData{
		BaseData: BaseData{
			User: u,
		},
		Group: g,
	})
}

func (a *App) updateGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	req, err := schemaDecode[groupPkg.UpdateRequest](r)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}
	req.Id = id

	err = a.group.Update(req)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/group/"+id, http.StatusSeeOther)
}

func (a *App) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := a.group.Delete(id)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/group/list", http.StatusSeeOther)
}

func (a *App) removeGroupMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userId := chi.URLParam(r, "userId")

	err := a.group.RemoveMember(id, userId)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/group/"+id, http.StatusSeeOther)
}

func (a *App) refreshInviteLinkGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := a.group.RefreshInviteId(id)
	if err != nil {
		a.renderErrorNotif(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Add("HX-Location", "/group/"+id)
	w.Write(nil)
}
