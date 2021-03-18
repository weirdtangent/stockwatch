var scripts = document.getElementsByTagName('script');
var lastScript = scripts[scripts.length-1];
var scriptName = lastScript;

var showing = 'tickerChart1';
var ticker = scriptName.getAttribute('data-ticker');
var exchange = scriptName.getAttribute('data-exchange');
var is_market_open = scriptName.getAttribute('data-is-market-open');
var quote_refresh = scriptName.getAttribute('data-quote-refresh');

function update_quote(count) {
  $('#auto_refresh_working').html('<i class="ms-2 mb-2 myyellow fad fa-pulse fa-signal-stream"></i>')

  var response = $.ajax({
    type: 'GET',
    url: '/api/v1/quote?symbol=' + ticker,
    async: true,
    success: function(response) {
      ['quote_shareprice', 'quote_ask', 'quote_asksize', 'quote_bid', 'quote_bidsize', 'quote_asof', 'quote_change', 'quote_change_pct'].forEach(function(item) {
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
      if (is_market_open && $('#is_market_open_color').hasClass("text-danger")) {
        $('#is_market_open_color').fadeOut().fadeIn('slow').removeClass("text-danger").addClass("text-success").fadeOut().fadeIn('slow')
        $('#is_market_open').delay(100).fadeOut().fadeIn('slow').text("TRADING").delay(100).fadeOut().fadeIn('slow')
      } else if (!is_market_open && $('#is_market_open_color').hasClass("text-success")) {
        $('#is_market_open_color').fadeOut().fadeIn('slow').removeClass("text-success").addClass("text-danger").fadeOut().fadeIn('slow')
        $('#is_market_open').delay(100).fadeOut().fadeIn('slow').text("CLOSED").delay(100).fadeOut().fadeIn('slow')
      }
      if (count == 0) {
        $('#auto_refresh').delay(100).fadeOut().fadeIn('slow').html('<i class="ms-2 mb-2 far fa-pause-circle"></i> paused').delay(100).fadeOut().fadeIn('slow')
      }
    },
    complete: function() {
      $('#auto_refresh_working').html('')
      if (count > 0) {
        if (is_market_open) {
          setTimeout(function() {
            update_quote(count-1);
          }, quote_refresh * 1000);
        } else {
          setTimeout(function() {
            update_quote(count-1);
          }, 15 * 60 * 1000);
        }
      }
    }
  });
}

$(document).ready(function() { 

  // at most, 30 updates: 10 min during open, 7.5 hours while closed
  setTimeout(function() {
    update_quote(30);
  }, quote_refresh * 1000);

  $('#auto_refresh').on('click', function() {
    $('#auto_refresh').delay(100).fadeOut().fadeIn('slow').html('<i class="ms-2 mb-2 fad fa-sync fa-spin"></i> ' + quote_refresh + ' sec</span>').delay(100).fadeOut().fadeIn('slow');
    update_quote(3);
  })

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
