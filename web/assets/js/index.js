import { initDB, saveOfflineRequest, retryPendingRequests } from './indexedDB.js';

initDB();

import UploadManager from './upload_manager.js';

window.addEventListener('resize', () => {
    if (document.body.clientWidth > 600) {
        document.querySelector('.mobile-nav').style.display = 'none';
    }
})

function showBanner(msg, color) {
    const banner = document.createElement("div");
    banner.textContent = msg;
    Object.assign(banner.style, {
        position: "fixed",
        bottom: "20px",
        left: "20px",
        padding: "10px",
        background: color,
        color: "#fff",
        borderRadius: "5px",
        fontSize: "14px",
        zIndex: 9999
    });
    document.body.appendChild(banner);
    setTimeout(() => banner.remove(), 5000);
}

window.addEventListener('online', () => {
    console.log("Network restored");
    retryPendingRequests();
    showBanner("Network restored", "green");
});

window.addEventListener('offline', () => {
    console.log("Network lost");
    showBanner("Network lost", "red");
});

function throttle(fn, delay) {
    let last = 0;
    return function () {
        const now = +new Date();
        if (now - last > delay) {
            fn.apply(this, arguments);
            last = now;
        }
    };
}

window.addcomment = throttle(addcomm, 5000);

window.postreaction = function (postId, reaction) {
    document.getElementById("errorlogin" + postId).innerText = ``;
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/post/postreaction", true);
    xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                const response = JSON.parse(xhr.responseText);
                document.getElementById("likescount" + postId).innerHTML = `${response.likesCount}`;
                document.getElementById("dislikescount" + postId).innerHTML = `${response.dislikesCount}`;
            } else if (xhr.status === 401) {
                document.getElementById(
                    "errorlogin" + postId
                ).innerText = `You must login first!`;
                setTimeout(() => {
                    document.getElementById("errorlogin" + postId).innerText = ``;
                }, 1000);
            } else if (xhr.status === 400) {
                document.getElementById(
                    "errorlogin" + postId
                ).innerText = `Bad request!`;
                setTimeout(() => {
                    document.getElementById("errorlogin" + postId).innerText = ``;
                }, 1000);
            } else if (xhr.status === 500) {
                document.getElementById(
                    "errorlogin" + postId
                ).innerText = `Try again later!`;
                setTimeout(() => {
                    document.getElementById("errorlogin" + postId).innerText = ``;
                }, 1000);
            }
        }
    };
    xhr.send(`reaction=${reaction}&post_id=${postId}`);
}


window.commentreaction = function (commentid, reaction) {
    document.getElementById("commenterrorlogin" + commentid).innerText = ``;
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/post/commentreaction", true);
    xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                const response = JSON.parse(xhr.responseText);
                document.getElementById("commentlikescount" + commentid).innerHTML = `${response.commentlikesCount}`;
                document.getElementById(
                    "commentdislikescount" + commentid
                ).innerHTML = `${response.commentdislikesCount}`;
            } else if (xhr.status === 401) {
                document.getElementById(
                    "commenterrorlogin" + commentid
                ).innerText = `You must login first!`;
                setTimeout(() => {
                    document.getElementById(
                        "commenterrorlogin" + commentid
                    ).innerText = ``;
                }, 1000);
            } else if (xhr.status === 400) {
                document.getElementById(
                    "commenterrorlogin" + commentid
                ).innerText = `bad request!`;
                setTimeout(() => {
                    document.getElementById(
                        "commenterrorlogin" + commentid
                    ).innerText = ``;
                }, 1000);
            } else if (xhr.status === 500) {
                document.getElementById(
                    "commenterrorlogin" + commentid
                ).innerText = `Try again later!`;
                setTimeout(() => {
                    document.getElementById(
                        "commenterrorlogin" + commentid
                    ).innerText = ``;
                }, 1000);
            }
        }
    };
    xhr.send(`reaction=${reaction}&comment_id=${commentid}`);
}

function addcomm(postId) {
    const content = document.getElementById("comment-content");
    const data = `postid=${postId}&comment=${encodeURIComponent(content.value)}`;

    if (navigator.onLine) {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", "/post/addcommentREQ", true);
        xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
        xhr.onreadystatechange = function () {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    const response = JSON.parse(xhr.responseText);
                    const comment = document.createElement("div")
                    comment.innerHTML = `<div class="comment">
                        <div class="comment-header">
                            <span class="comment-author">` + response.username + `</span>
                            <span class="comment-date">` + response.created_at + `</span>
                        </div>
                        <div class="comment-body">` + response.content + `</div>
                        <div class="comment-footer">
                            <button onclick="commentreaction(` + response.comment_id + `, 1)">👍 <span id="commentlikescount` + response.comment_id + `">` + response.likes + `</span></button>
                            <button onclick="commentreaction(` + response.comment_id + `, 0)">👎 <span id="commentdislikescount` + response.comment_id + `">` + response.dislikes + `</span></button>
                        </div>
                        <span id="commenterrorlogin` + response.comment_id + `"> </span>
                    </div>`
                    document.getElementsByClassName("comments")[0].prepend(comment)
                    document.getElementsByClassName("post-comments")[0].innerHTML = `` + response.commentscount
                    content.value = ""
                } else if (xhr.status === 400) {
                    document.getElementById("errorlogin" + postId).innerText = `Invalid comment!`
                    setTimeout(() => {
                        document.getElementById("errorlogin" + postId).innerText = ``
                    }, 1000);
                } else if (xhr.status === 401) {
                    document.getElementById("errorlogin" + postId).innerText = `You must login first!`
                    setTimeout(() => {
                        document.getElementById("errorlogin" + postId).innerText = ``
                    }, 1000);
                } else {
                    document.getElementById("errorlogin" + postId).innerText = `Cannot add comment now, try again later!`
                    setTimeout(() => {
                        document.getElementById("errorlogin" + postId).innerText = ``
                    }, 1000);
                }
            }
        }
        xhr.send(data);
    } else {
        saveOfflineRequest('/post/addcommentREQ', data);
        content.value = '';
    }
}
window.addcomm = addcomm;

const select = document.getElementById("categories-select");
if (select) {
    select.addEventListener("change", (e) => {
        // Parse the value as JSON to extract id and label
        const selectedValue = JSON.parse(e.target.value);
        const { id, label } = selectedValue;

        // create the elemenet for the category
        const span = document.createElement("span");
        span.textContent = label;
        span.classList.add("selected-category");

        // Add a remove button to the span
        const removeBtn = document.createElement("span");
        removeBtn.textContent = "×";
        removeBtn.classList.add("remove-category");
        removeBtn.addEventListener("click", () => {
            span.remove();
            input.remove();

            // Re-enable the corresponding option in the select
            Array.from(e.target.options).find((option) => {
                try {
                    const optionValue = JSON.parse(option.value);
                    return optionValue.id === id;
                } catch {
                    return false;
                }
            }).disabled = false;
        });
        span.appendChild(removeBtn);

        // create hidden input to hold the id of selected category
        const input = document.createElement("input");
        input.type = "hidden";
        input.value = id;
        input.name = "categories";

        // add the elements (span and hidden input)
        // at the first position of the categories container
        const categoriesContainer = document.querySelector(".selected-categories");
        categoriesContainer.append(input, span);

        // disable the option selected in the select
        e.target.options[e.target.selectedIndex].disabled = true;

        // Reset the select
        e.target.selectedIndex = 0;
    });
}

window.pagination = async function (dir, data) {
    const path = window.location.pathname;
    if (dir === "next" && data) {
        const page = +document.querySelector(".currentpage").innerText + 1;
        window.location.href = path + "?PageID=" + page;
    }
    if (
        dir === "back" &&
        document.querySelector(".currentpage").innerText > "1"
    ) {
        const page = +document.querySelector(".currentpage").innerText - 1;
        window.location.href = path + "?PageID=" + page;
    }
}

window.CreatPost = async function () {
  const title = document.querySelector(".create-post-title");
  const content = document.querySelector(".content");
  const categories = document.querySelector(".selected-categories");
  const logerror = document.querySelector(".errorarea");
  const btn = document.getElementById("create-post-btn");
  const imageInput = document.getElementById("post-image");

  if (!title || !content || !categories || !logerror) return;

  if (!title.value || !content.value || categories.childElementCount === 0) {
      logerror.innerText = "Please fill in all fields and select at least one category.";
      setTimeout(() => {
          logerror.innerText = "";
      }, 3000);
      return;
  }

  if (title.value.length > 100) {
      logerror.innerText = "Title is too long. Please keep it under 100 characters.";
      setTimeout(() => {
          logerror.innerText = "";
      }, 3000);
      return;
  }

  if (content.value.length > 3000) {
      logerror.innerText = "Content is too long. Please keep it under 3000 characters.";
      setTimeout(() => {
          logerror.innerText = "";
      }, 3000);
      return;
  }

  // Disable button and show spinner
  document.getElementById("publish-post-icon").style.display = "none";
  document.getElementById("publish-post-circle").style.display = "inline-block";
  btn.disabled = true;
  btn.style.background = "grey";
  btn.style.cursor = "not-allowed";

  let imageURL = "";
  if (imageInput.files.length > 0) {
    const file = imageInput.files[0];
    const uploadManager = new UploadManager();

    // Update UI with progress
    uploadManager.setProgressCallback((percent, message) => {
      logerror.innerText = `${message} (${percent}%)`;
      logerror.style.color = "blue";
    });

    try {
      const result = await uploadManager.upload(file);
      imageURL = result.imageURL;
      logerror.innerText = "Upload ảnh thành công!";
      logerror.style.color = "green";
    } catch (error) {
      logerror.innerText = `Lỗi upload: ${error.message}`;
      logerror.style.color = "red";

      // Reset button
      document.getElementById("publish-post-icon").style.display = "inline-block";
      document.getElementById("publish-post-circle").style.display = "none";
      btn.disabled = false;
      btn.style.background = "";
      btn.style.cursor = "pointer";
      return;
    }
  }

  let cateris = new Array();
  Array.from(categories.getElementsByTagName("input")).forEach((x) => {
    cateris.push(x.value);
  });

  const formData = new FormData();

  if (navigator.onLine) {
    const xml = new XMLHttpRequest();
    xml.open("POST", "/post/createpost", true);

    formData.append("title", title.value);
    formData.append("content", content.value);
    cateris.forEach(catID => formData.append("categories", catID));

    if (imageURL) {
      formData.append("image_url", imageURL);
    }

    xml.onreadystatechange = function () {
      if (xml.readyState === 4) {
        if (xml.status === 200) {
          const btn = document.getElementById("create-post-btn");
          document.getElementById("publish-post-icon").style.display = "none";
          document.getElementById("publish-post-circle").style.display =
            "inline-block";
          btn.disabled = true;
          btn.style.background = "grey";
          btn.style.cursor = "not-allowed";

          if (imageURL) {
          logerror.innerText = "Post created! Verifying image availability...";
          logerror.style.color = "blue";

          // Poll for image availability
          let attempts = 0;
          const maxAttempts = 20; // 10 seconds

          const checkImage = setInterval(() => {
            attempts++;
            fetch(imageURL, { method: 'HEAD' })
              .then(res => {
                if (res.ok || attempts >= maxAttempts) {
                  clearInterval(checkImage);
                  logerror.innerText = "Image ready! Redirecting...";
                  logerror.style.color = "green";
                  setTimeout(() => {
                    window.location.href = "/";
                  }, 500);
                }
              })
              .catch(() => {
                if (attempts >= maxAttempts) {
                  clearInterval(checkImage);
                  window.location.href = "/";
                }
              });
          }, 500);
        } else {
          logerror.innerText = "Post created successfully, redirect to home page in 2s ...";
          logerror.style.color = "green";
          setTimeout(() => {
            window.location.href = "/";
          }, 2000);
        } 
      } else if (xml.status === 401) {
          logerror.innerText = "You are loged out, redirect to login page in 2s...";
          setTimeout(() => {
            window.location.href = "/login";
          }, 2000);
        } else {
          logerror.innerText = "Error: check your entries and try again!";
          setTimeout(() => {
            logerror.innerText = "";
          }, 1500);

          // Reset button on error
          document.getElementById("publish-post-icon").style.display = "inline-block";
          document.getElementById("publish-post-circle").style.display = "none";
          btn.disabled = false;
          btn.style.background = "";
          btn.style.cursor = "pointer";
        }
      }
    };

    // Get form data
    xml.send(formData);
  } else {
      saveOfflineRequest('/post/createpost', data);
      setTimeout(() => {
          window.location.href = '/';
      }, 2000);
  }  
}

window.register = function () {
    const email = document.querySelector("#email");
    const username = document.querySelector("#username");
    const password = document.querySelector("#password");
    const passConfirm = document.querySelector("#password-confirmation");

    const xml = new XMLHttpRequest();
    xml.open("POST", "/signup", true);
    xml.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
    xml.onreadystatechange = function () {
        if (xml.readyState === 4) {
            const logerror = document.querySelector(".errorarea");
            if (xml.status === 200) {
                logerror.innerText = `User ${username.value} created successfully, redirect to login page in 2s ...`;
                logerror.style.color = "green";
                setTimeout(() => {
                    window.location.href = "/login";
                }, 2000);
            } else if (xml.status === 302) {
                logerror.innerText =
                    "You are already loged in, redirect to home page in 2s...";
                logerror.style.color = "green";
                setTimeout(() => {
                    window.location.href = "/";
                }, 2000);
            } else if (xml.status === 400) {
                logerror.innerText = "Error: verify your data and try again!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            } else if (xml.status === 304) {
                logerror.innerText = "User already exists!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            } else {
                logerror.innerText = "Cannot create user, try again later!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            }
        }
    };
    // Get form data
    xml.send(
        `email=${encodeURIComponent(email.value)}&username=${encodeURIComponent(
            username.value
        )}&password=${encodeURIComponent(
            password.value
        )}&password-confirmation=${encodeURIComponent(passConfirm.value)}`
    );
}

window.login = function () {
    const username = document.querySelector("#username");
    const password = document.querySelector("#password");

    const xml = new XMLHttpRequest();
    xml.open("POST", "/signin", true);
    xml.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
    xml.onreadystatechange = function () {
        if (xml.readyState === 4) {
            const logerror = document.querySelector(".errorarea");
            if (xml.status === 200) {
                logerror.innerText = `Login in successfully, redirect to home page in 2s ...`;
                logerror.style.color = "green";
                setTimeout(() => {
                    window.location.href = "/";
                }, 2000);
            } else if (xml.status === 302) {
                logerror.innerText =
                    "You are already loged in, redirect to home page in 2s...";
                logerror.style.color = "green";
                setTimeout(() => {
                    window.location.href = "/";
                }, 2000);
            } else if (xml.status === 400) {
                logerror.innerText = "Error: verify your data and try again!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            } else if (xml.status === 404) {
                logerror.innerText = "User not found!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            } else if (xml.status === 401) {
                logerror.innerText = "Invalid username or password!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            } else {
                logerror.innerText = "Cannot log you in now, try again later!";
                logerror.style.color = "red";
                setTimeout(() => {
                    logerror.innerText = "";
                }, 1500);
            }
        }
    };
    // Get form data
    xml.send(
        `username=${encodeURIComponent(
            username.value
        )}&password=${encodeURIComponent(password.value)}`
    );
}

window.displayMobileNav = (e) => {
    const nav = document.querySelector(".mobile-nav");
    nav.style.display = "block";
};

window.closeMobileNav = (e) => {
    const nav = document.querySelector(".mobile-nav");
    nav.style.display = "none";
};

// const formatTime = (timeStr) => {
//     // Parse the input time string
//     const date = new Date(timeStr);
//     return date.toLocaleString('default', {
//         hour: '2-digit',
//         minute: '2-digit',
//         day: '2-digit',
//         month: '2-digit',
//         year: 'numeric',
//     }).replace(',', ' ')
// }

// document.addEventListener("DOMContentLoaded", () => {
//     document.querySelectorAll("[data-timestamp]").forEach((element) => {
//         const time = element.getAttribute("data-timestamp");
//         if (time) {
//             element.textContent = formatTime(time);
//         }
//     });
// });