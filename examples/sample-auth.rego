package auth

default allow = false

tokens := {
    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2Njk5NjQ0ODh9.elDm_t4vezVgEmS8UFFo_spLJTts7JWybzbyO_aYV3Y",
}

allow {
    tokens[input.token]
}
