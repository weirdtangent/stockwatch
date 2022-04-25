$(document).ready(function() {
    window.onAmazonLoginReady = function() {
        amazon.Login.setClientId('amzn1.application-oa2-client.857a9789628d4607b27bf7a98119f7c1');
      };
      (function(d) {
        var a = d.createElement('script'); a.type = 'text/javascript';
        a.async = true; a.id = 'amazon-login-sdk';
        a.src = 'https://assets.loginwithamazon.com/sdk/na/login1.js';
        d.getElementById('amazon-root').appendChild(a);
      })(document);

    document.getElementById('LoginWithAmazon').onclick = function() {
        options = {}
        options.scope = 'profile';
        options.scope_data = { 'profile' : {'essential': false} };
        amazon.Login.authorize(options, 'https://stockwatch.graystorm.com/auth/amazon');
        return false;
    };
});
