# Garcon Examples

The sub-folders [chi](chi) and [httprouter](httprouter)
compare two HTTP routers:

- <https://github.com/go-chi/chi>
- <https://github.com/julienschmidt/httprouter>

The idea is to handle `POST /` request in `post()` function,
while all other requests are processed by `others()`.

The original motivation was to report an issue:
<https://github.com/go-chi/chi/issues/738>
