package app

import (
	"net/http"
	"time"

	groupPkg "github.com/mattfan00/jvbe/group"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()

	publicFileServer := http.FileServer(http.Dir("./ui/public"))
	r.Handle("/public/*", http.StripPrefix("/public/", publicFileServer))

	r.Get("/privacy", a.renderPrivacy())

	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitAll(100, 1*time.Minute))
		r.Use(middleware.Logger)
		r.Use(a.recoverPanic)
		r.Use(a.session.LoadAndSave)

		r.Get("/", a.renderIndex())

		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", a.renderLogin())
			r.Get("/callback", a.handleLoginCallback())

			r.With(a.requireAuth).Get("/logout", a.handleLogout())
		})

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)

			r.Get("/home", a.renderHome())
			r.With(a.canDoEverything).Get("/admin", a.renderAdmin)

			r.Route("/event", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(a.canModifyEvent)

					r.Get("/new", a.renderNewEvent())
					r.Post("/new", a.createEvent())
					r.Get("/{id}/edit", a.renderEditEvent())
					r.Post("/{id}/edit", a.updateEvent())
					r.Delete("/{id}/edit", a.deleteEvent())
				})

				r.Get("/{id}", a.renderEventDetails())
				r.Post("/respond", a.respondEvent())
			})
		})

		r.Route("/group", func(r chi.Router) {
			r.Get("/{id}/invite", a.inviteGroup)

			r.Group(func(r chi.Router) {
				r.Use(a.requireAuth)

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

				r.Get("/{id}", a.renderGroupDetails)
			})
		})

		r.Route("/review", func(r chi.Router) {
			r.Get("/request", a.renderReviewRequest())
			r.Post("/request", a.updateReview())

			r.Group(func(r chi.Router) {
				r.Use(a.canReviewUser)

				r.Get("/list", a.renderReviewList())
				r.Post("/approve", a.approveReview())
			})
		})

	})

	return r
}

func (a *App) renderPrivacy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.renderPage(w, "privacy.html", nil)
	}
}

func (a *App) renderIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.renderPage(w, "index.html", nil)
	}
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
		a.session.Put(r.Context(), "redirect", r.URL.String())
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
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

func (a *App) renderAdmin(w http.ResponseWriter, r *http.Request) {
	u, _ := a.sessionUser(r)

	a.renderPage(w, "admin.html", BaseData{
		User: u,
	})
}
