package routes

import (
	"database/sql"
	"net/http"

	"forum/server/config"
	"forum/server/controllers"
	"forum/server/middleware"
)

func Routes(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// Initialize rate limit config
	rateLimitConfig := config.DefaultRateLimitConfig()

	// Initialize rate limiting middleware
	endpointLimiter := middleware.NewEndpointRateLimiter(db, rateLimitConfig)

	// serve static files (no rate limiting on assets)
	mux.HandleFunc("/assets/", controllers.ServeStaticFiles)

	// routes to get pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		controllers.IndexPosts(w, r, db)
	})
	
	mux.HandleFunc("/category/{id}", func(w http.ResponseWriter, r *http.Request) {
		controllers.IndexPostsByCategory(w, r, db)
	})
	mux.HandleFunc("/mycreatedposts", func(w http.ResponseWriter, r *http.Request) {
		controllers.MyCreatedPosts(w, r, db)
	})
	
	mux.HandleFunc("/mylikedposts", func(w http.ResponseWriter, r *http.Request) {
		controllers.MyLikedPosts(w, r, db)
	})
	mux.HandleFunc("/post/{id}", func(w http.ResponseWriter, r *http.Request) {
		controllers.ShowPost(w, r, db)
	})

	// Rate limited comment creation
	mux.HandleFunc("/post/addcommentREQ", 
		endpointLimiter.LimitCreateComment(func(w http.ResponseWriter, r *http.Request) {
			controllers.CreateComment(w, r, db)
		}, db),
	)

	mux.HandleFunc("/post/create", func(w http.ResponseWriter, r *http.Request) {
		controllers.GetPostCreationForm(w, r, db)
	})

	// Rate limited post creation
	mux.HandleFunc("/post/createpost", 
		endpointLimiter.LimitCreatePost(func(w http.ResponseWriter, r *http.Request) {
			controllers.CreatePost(w, r, db)
		}, db),
	)

	// Rate limited reactions
	mux.HandleFunc("/post/postreaction", func(w http.ResponseWriter, r *http.Request) {
			controllers.ReactToPost(w, r, db)
		})

	mux.HandleFunc("/post/commentreaction", func(w http.ResponseWriter, r *http.Request) {
			controllers.ReactToComment(w, r, db)
		})

	// Rate limited login
	mux.HandleFunc("/signin", 
		endpointLimiter.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
			controllers.Signin(w, r, db)
		}, db),
	)

	// Rate limited signup
	mux.HandleFunc("/signup", 
		endpointLimiter.LimitRegister(func(w http.ResponseWriter, r *http.Request) {
			controllers.Signup(w, r, db)
		}, db),
	)

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		controllers.GetLoginPage(w, r, db)
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		controllers.Logout(w, r, db)
	})


	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		controllers.GetRegisterPage(w, r, db)
	})

	return mux
}
