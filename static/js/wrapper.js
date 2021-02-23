var validated=0;

function init() {
  gapi.load('auth2', function() {
    /* Ready. Make a call to gapi.auth2.init or some other API */
  });

  setTimeout(() => {
    if (!validated) {
      $(".hideOnProfile").show();
    }
  } , 2000);

}

function signOut() {
  var auth2 = gapi.auth2.getAuthInstance();
  auth2.signOut().then(function () {
    console.log('User signed out.');
  });
  $(".hideOnProfile").show();
  $(".showOnProfile").hide();
}

function onFailure(error) {
  $(".hideOnProfile").show();
  $(".showOnProfile").hide();
}

function onSignIn(googleUser) {
  var profile = googleUser.getBasicProfile();
  var id_token = googleUser.getAuthResponse().id_token;
  var xhr = new XMLHttpRequest();
  xhr.open('POST', 'https://stockwatch.graystorm.com/tokensignin');
  xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
  xhr.onload = function() {
    onValidated(profile)
    //console.log('Signed in as: ' + xhr.responseText);
  };
  xhr.send('idtoken=' + id_token);
}

function onValidated(profile) {
  validated=1;
  $(".hideOnProfile").hide();
  $(".showOnProfile").show();
  $(".profileName").html(profile.getName());
  $(".profileImage").attr("src", profile.getImageUrl());
}


$(document).ready(function() {
  $(".signout").each(
    addEventListener('click', function() {
      signOut();
    }, false));
});
