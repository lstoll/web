# TODO for dbsc

* Rather than explicit challenges, maybe use two cookies. existing stays long lived, contains session ID. Short-lived token is the "current session id". We can store that pn the session, and use that to validate we are on the actual machine between refreshes.
* Look at the authorization header a bit more - work out the diff from challenge, maybe it's useful
* Cookie session support - can store the DBSC key in the encrypted token.
