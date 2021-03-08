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


  $("input[name=pickTimespan]").on("change", function() {
    console.log("radio button clicked")
    href = $("input[name=pickTimespan]:checked").data("href")
    console.log(href)
    window.document.location = href;
  });

  Date.prototype.toDateInputValue = (function() {
    var local = new Date(this);
    local.setMinutes(this.getMinutes() - this.getTimezoneOffset());
    return local.toJSON().slice(0,10);
  });

  $('#PurchaseDate').val(new Date().toDateInputValue());
});
