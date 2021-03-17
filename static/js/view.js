var showing = 'tickerChart1';
var symbol = document.currentScript.getAttribute('symbol');
var acronym = document.currentScript.getAttribute('acronym');
var is_market_open = document.currentScript.getAttribute('is_market_open');
var quote_refresh = document.currentScript.getAttribute('quote_refresh');

function update_quote() {
  setTimeout(function() {
    $('.hideTillComplete').show()
  }, 1 * 1000);

  var response = $.ajax({
    type: 'GET',
    url: '/api/v1/quote?symbol=' + symbol + '&acronym=' + acronym,
    async: true,
    success: function(response) {
      ['quote_shareprice', 'quote_ask', 'quote_bid', 'quote_asof'].forEach(function(item) {
        if ($('#' + item).text() != response['data'][item]) {
          ($('#' + item).delay(100).fadeOut().fadeIn('slow').text(response['data'][item])).delay(100).fadeOut().fadeIn('slow')
        }
      });
      if (response.data.quote_dailymove === 'down' && $('#quote_dailymove').hasClass("fa-arrow-up")) {
        $('#quote_dailymove').fadeOut().fadeIn('slow').removeClass("fa-arrow-up text-success").addClass("fa-arrow-down text-danger").fadeOut().fadeIn('slow')
      } else if (response.data.quote_dailymove === 'up' && $('#quote_dailymove').hasClass("fa-arrow-down")) {
        $('#quote_dailymove').fadeOut().fadeIn('slow').removeClass("fa-arrow-down text-danger").addClass("fa-arrow-up text-success").fadeOut().fadeIn('slow')
      }
      is_market_open = response.data.is_market_open
    },
    complete: function() {
      if (is_market_open) {
        setTimeout(function() {
          update_quote();
        }, quote_refresh * 1000);
      }
    }
  });
}

$(document).ready(function() { 

  if (is_market_open) {
    update_quote();
  }

  $('input[name=pickChart]').each(function (e) {
    elem = $('#' + this.id + 'elem');
    if (elem.length == 0) {
      this.remove()
    }
  });


  $('input[name=pickChart]').on('change', function() {
    clicked = $('input[name=pickChart]:checked').attr('id');

    $('#' + showing + 'elem').fadeOut('fast', function() {
      $('#' + clicked + 'elem').fadeIn('fast');
    });

    showing = clicked;
  });


  $('input[name=pickTimespan]').on('change', function() {
    console.log('radio button clicked')
    href = $('input[name=pickTimespan]:checked').data('href')
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
