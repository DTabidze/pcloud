package rest

import (
	"encoding/json"
	"net/http"

	"github.com/giolekva/pcloud/core/kg/model"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func (router *Router) initUsers() {
	router.Users.Handle("", router.buildCreateUserHandler()).Methods("POST")
	router.Users.Handle("", router.buildGetUsersHandler()).Methods("GET")
	router.User.Handle("", router.buildGetUserHandler()).Methods("GET")
}

func (router *Router) buildCreateUserHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		router.Logger.Debug("Rest API: create user")
		var user *model.User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			return errors.Wrap(err, "can't decode request body")
		}
		user.SanitizeInput()
		updatedUser, err := router.App.CreateUser(user)
		if err != nil {
			return errors.Wrap(err, "can't create user")
		}
		updatedUser.SanitizeOutput()

		jsoner(w, http.StatusOK, updatedUser)
		return nil
	}
	return HandlerFunc(fn)
}

func (router *Router) buildGetUsersHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		router.Logger.Debug("Rest API: get users")
		return nil
	}
	return HandlerFunc(fn)
}

func (router *Router) buildGetUserHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		router.Logger.Debug("Rest API: get user")
		params := mux.Vars(r)

		var userID string
		var ok bool
		if userID, ok = params["user_id"]; !ok {
			return errors.New("missing parameter: user_id")
		}
		user, err := router.App.GetUser(userID)

		if err != nil {
			return errors.Wrapf(err, "can't get user from app")
		}

		jsoner(w, http.StatusOK, user)
		return nil
	}
	return HandlerFunc(fn)
}
