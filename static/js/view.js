var showing = "tickerChart1";

$(document).ready(function() { 
  $("input[name=pickChart]").each(function (e) {
    elem = $('#' + this.id + 'elem');
    if (elem.length == 0) {
      this.remove()
    }
  });


  $("input[name=pickChart]").on("change", function() {
    clicked = $("input[name=pickChart]:checked").attr("id");

    $('#' + showing + "elem").fadeOut('fast', function() {
      $('#' + clicked + "elem").fadeIn('fast');
    });

    showing = clicked;
  });
});
