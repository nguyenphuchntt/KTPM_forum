package routes

import (
	"database/sql"
	"net/http"

	"forum/server/cloud"
	"forum/server/config"
	"forum/server/controller"
	"forum/server/middleware"
	"forum/server/usecase"
)

// Routes builds the mux. storage is nil when object storage is not configured,
// in which case the upload and media routes are left unregistered.
func Routes(db *sql.DB, storageConfig *config.MinIOConfig, storage cloud.Storage) http.Handler {
	mux := http.NewServeMux()

	// Layered (JSON API v1) dependencies and routes.
	authUc := usecase.NewAuthUsecase(db)
	postUc := usecase.NewPostUsecase(db, authUc)
	commentUc := usecase.NewCommentUsecase(db, authUc)
	authC := controllers.NewAuthController(authUc)
	postC := controllers.NewPostController(postUc)
	commentC := controllers.NewCommentController(commentUc)
	controllers.RegisterAuthRoutes(mux, authC)
	controllers.RegisterCommentRoutes(mux, commentC)
	controllers.RegisterPostRoutes(mux, postC)

	// Initialize rate limit config
	rateLimitConfig := config.DefaultRateLimitConfig()

	// Initialize rate limiting middleware
	endpointLimiter := middleware.NewEndpointRateLimiter(db, rateLimitConfig)

	// serve static files (no rate limiting on assets)
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		controllers.ServeStaticFiles(db, w, r)
	})

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

	// Post creation form page
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

	mux.HandleFunc("/post/addcommentREQ", func(w http.ResponseWriter, r *http.Request) {
		commentC.CreateCommentSSR(w, r)
	})

	mux.HandleFunc("/post/commentreaction", func(w http.ResponseWriter, r *http.Request) {
		commentC.ReactToCommentSSR(w, r)
	})

	// Delete post route
	mux.HandleFunc("/post/delete/{id}", func(w http.ResponseWriter, r *http.Request) {
		controllers.DeletePost(w, r, db)
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

	// Image upload (valet key). Both endpoints are absent when object storage is
	// not configured, so the app degrades instead of failing requests later.
	if storage != nil {
		uploadUc := usecase.NewUploadUsecase(db, storage, storageConfig, authUc)
		uploadC := controllers.NewUploadController(uploadUc)
		mediaC := controllers.NewMediaController(uploadUc)

		// One upload costs two requests here (ticket + confirm), which is why
		// the shared bucket is sized for twice the intended uploads per minute.
		mux.Handle("/api/upload/request-url",
			endpointLimiter.LimitUpload(http.HandlerFunc(uploadC.RequestUploadURLJSON), db))
		mux.Handle("/api/upload/confirm",
			endpointLimiter.LimitUpload(http.HandlerFunc(uploadC.ConfirmUploadJSON), db))

		// This is the route model.Post.ImagePath has always pointed <img src> at.
		controllers.RegisterMediaRoutes(mux, mediaC)
	}

	return mux
}
