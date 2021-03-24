var scripts = document.getElementsByTagName('script');
var lastScript = scripts[scripts.length-1];
var scriptName = lastScript;

var showing = 'tickerChart1';
var ticker = scriptName.getAttribute('data-ticker');
var exchange = scriptName.getAttribute('data-exchange');
var is_market_open = scriptName.getAttribute('data-is-market-open') === "true";
var quote_refresh = scriptName.getAttribute('data-quote-refresh');

function update_quote(count) {

  var response = $.ajax({
    type: 'GET',
    url: '/api/v1/quote?symbol=' + ticker,
    async: true,
    success: function(response) {
      ['quote_shareprice', 'quote_ask', 'quote_asksize', 'quote_bid', 'quote_bidsize', 'quote_asof', 'quote_change', 'quote_change_pct'].forEach(function(item) {
        if ($('#' + item).text() != response['data'][item]) {
          $('#' + item).animate({opacity: 0}, 400, function() { $('#' + item).text(response['data'][item]).animate({opacity: 1}, 400) });
        }
      });
      if (response.data.quote_dailymove === 'down' && $('#quote_dailymove').hasClass("fa-arrow-up")) {
        $('#quote_dailymove_text').animate({opacity: 0}, 400, function() { $('#quote_dailymove_text').removeClass("text-success").addClass("text-danger").animate({opacity: 1}, 400) });
        $('#quote_dailymove').animate({opacity: 0}, 400, function() { $('#quote_dailymove').removeClass("fa-arrow-up").addClass("fa-arrow-down").animate({opacity: 1}, 400) });
      } else if (response.data.quote_dailymove === 'up' && $('#quote_dailymove').hasClass("fa-arrow-down")) {
        $('#quote_dailymove_text').animate({opacity: 0}, 400, function() { $('#quote_dailymove_text').removeClass("text-danger").addClass("text-success").animate({opacity: 1}, 400) });
        $('#quote_dailymove').animate({opacity: 0}, 400, function() { $('#quote_dailymove').removeClass("fa-arrow-down text-danger").addClass("fa-arrow-up text-success").animate({opacity: 1}, 400) });
      }
      is_market_open = response.data.is_market_open === true;
      if (is_market_open && $('#is_market_open_color').hasClass("text-danger")) {
        $("#ticker_quote_info").show();
        $("#ticker_eod_info").hide();
        $('#is_market_open_color').animate({opacity: 0}, 400, function() { $('#is_market_open_color').removeClass("text-danger").addClass("text-success").animate({opacity: 1}, 400) });
        $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text("TRADING").animate({opacity: 1}, 400) });
        $('#auto_refresh_working').html('<i class="ms-2 mb-2 myyellow fad fa-pulse fa-signal-stream"></i>')
      } else if (!is_market_open && $('#is_market_open_color').hasClass("text-success")) {
        $("#ticker_quote_info").hide();
        $("#ticker_eod_info").show();
        $('#is_market_open_color').animate({opacity: 0}, 400, function() { ($('#is_market_open_color').removeClass("text-success").addClass("text-danger").animate({opacity: 1}, 400)) });
        $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text("CLOSED").animate({opacity: 1}, 400) });
        $('#auto_refresh').html('<i class="ms-2 mb-2 far fa-pause-circle"></i> paused');
      }
      if (count == 0) {
        $('#auto_refresh').html('<i class="ms-2 mb-2 far fa-pause-circle"></i> paused');
      } else if ($('#auto_refresh').html() == "") {
        $('#auto_refresh').html('<i class="ms-2 mb-2 fad fa-sync fa-spin"></i> ' + quote_refresh + 'sec');
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

  if (is_market_open) {
    $("#ticker_quote_info").show();
    $("#auto_refresh").show();
  } else {
    $("#ticker_eod_info").show();
  }

  $('#auto_refresh').on('click', function() {
    $('#auto_refresh').animate({opacity: 0}, 400, function() { ($('#auto_refresh').html('<i class="ms-2 mb-2 fad fa-sync fa-spin"></i> ' + quote_refresh + ' sec</span>').animate({opacity: 1}, 400)) });
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
