package controllers

import (
	"net/http"

	"forum/server/usecase"
)

type PostController struct {
	Post *usecase.PostUsecase
}

func NewPostController(post *usecase.PostUsecase) PostController {
	return PostController{Post: post}
}

func RegisterPostRoutes(mux *http.ServeMux, c PostController) {
	mux.HandleFunc("/api/v1/posts", c.PostsCollectionJSON)
	mux.HandleFunc("/api/v1/posts/{id}", c.PostItemJSON)
	mux.HandleFunc("/api/v1/posts/{id}/reactions", c.ReactToPostJSON)
	mux.HandleFunc("/api/v1/me/posts", c.MyPostsJSON)
	mux.HandleFunc("/api/v1/me/liked-posts", c.MyLikedPostsJSON)
}
