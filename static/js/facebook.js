window.fbAsyncInit = function() {
    FB.init({
      appId      : '698068408065680',
      cookie     : true,
      xfbml      : true,
      version    : 'v13.0',
      redirect_url : 'https://stockwatch.graystorm.com/auth/facebook',
      redirect_uri : 'https://stockwatch.graystorm.com/auth/facebook'
    });
    FB.AppEvents.logPageView();
  };

  (function(d, s, id){
    var js, fjs = d.getElementsByTagName(s)[0];
    if (d.getElementById(id)) {return;}
    js = d.createElement(s); js.id = id;
    js.src = "https://connect.facebook.net/en_US/sdk.js";
    fjs.parentNode.insertBefore(js, fjs);
  }(document, 'script', 'facebook-jssdk'));

  function checkLoginState() {
    FB.getLoginStatus(function(response) {
      console.log(response);
    });
  }