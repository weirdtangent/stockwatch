function initGSO() {
  gapi.load('auth2', function() {
    console.log("gapi loaded and ready");
    $('.signout').on('click', function() { 
      gapi.auth2.init();
      var auth2 = gapi.auth2.getAuthInstance();
      if (auth2) {
        console.log("found auth instance");
        auth2.signOut().then(function () {
          console.log("signed out, redirecting");
          document.location.href = "https://stockwatch.graystorm.com/?signout=1";
        });
      } else {
        console.log("gapi isn't responding");
      }
    });
  });
}

function onSignIn(googleUser) {
  var id_token = googleUser.getAuthResponse().id_token;
  var xhr = new XMLHttpRequest();
  xhr.open('POST', 'https://stockwatch.graystorm.com/tokensignin');
  xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
  xhr.onload = function() {
    if (window.location.pathname === "/") {
      document.location.href = "/desktop";
    }
  };
  xhr.send('idtoken=' + id_token);
}

