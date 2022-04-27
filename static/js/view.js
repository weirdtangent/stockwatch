var scripts = document.getElementsByTagName('script');
var lastScript = scripts[scripts.length-1];
var scriptName = lastScript;

var showing = 'tickerChart1';
var ticker = scriptName.getAttribute('data-ticker');
var exchange = scriptName.getAttribute('data-exchange');
var is_market_open = scriptName.getAttribute('data-is-market-open') === "true";
var quote_refresh = scriptName.getAttribute('data-quote-refresh');

// every 20 sec for 1 hour = 180 refreshes
var update_count = 180

function update_quote() {
  if (update_count <= 0) {
    return;
  }
  $('#auto_refresh_working').removeClass('hide');
  var response = $.ajax({
      type: 'GET',
      url: '/api/v1/quote?symbol=' + ticker,
      async: true,
      success: function(response) {
          is_market_open = response.data.is_market_open === true;
          if (is_market_open) {
              ['quote_shareprice', 'quote_ask', 'quote_asksize', 'quote_bid', 'quote_bidsize', 'quote_asof', 'quote_change', 'quote_change_pct'].forEach(function(item) {
                  htmlId = '#' + item;
                  dataId = item;
                  if ($(htmlId).length && $(htmlId).text() != response['data'][dataId]) {
                      $(htmlId).animate({opacity: 0}, 400, function() {
                          $(htmlId).text(response['data'][dataId]).animate({opacity: 1}, 400) });
                  }
              });
              if (response.data.quote_dailymove === 'down' && !$('#quote_dailymove').hasClass("fa-arrow-down")) {
                  $('#quote_dailymove_text').animate({opacity: 0}, 400, function() { $('#quote_dailymove_text').removeClass("text-success").addClass("text-danger").animate({opacity: 1}, 400) });
                  $('#quote_dailymove').animate({opacity: 0}, 400, function() { $('#quote_dailymove').removeClass("fa-arrow-up text-success").addClass("fa-arrow-down").animate({opacity: 1}, 400) });
              } else if (response.data.quote_dailymove === 'up' && !$('#quote_dailymove').hasClass("fa-arrow-up")) {
                  $('#quote_dailymove_text').animate({opacity: 0}, 400, function() { $('#quote_dailymove_text').removeClass("text-danger").addClass("text-success").animate({opacity: 1}, 400) });
                  $('#quote_dailymove').animate({opacity: 0}, 400, function() { $('#quote_dailymove').removeClass("fa-arrow-down text-danger").addClass("fa-arrow-up text-success").animate({opacity: 1}, 400) });
              } else if (response.data.quote_dailymove === 'unchanged' && !$('#quote_dailymove').hasClass("fa-equals")) {
                  $('#quote_dailymove_text').animate({opacity: 0}, 400, function() { $('#quote_dailymove_text').removeClass("text-danger").removeClass("text-success").animate({opacity: 1}, 400) });
                  $('#quote_dailymove').animate({opacity: 0}, 400, function() { $('#quote_dailymove').removeClass("fa-arrow-down text-danger").removeClass("fa-arrot-up text-success").addClass("fa-equals").animate({opacity: 1}, 400) });
              }
          }
          if (response.data.last_updated_news != $('#last_updated_news').text()) {
            $('#last_updated_news').animate({opacity: 0}, 400, function() { $('#last_updated_news').text(response.data.last_updated_news).animate({opacity: 1}, 400) });
          }
          if (response.data.updating_news_now && $('#updating_news_now').hasClass('hide')) {
            $('#updating_news_now').removeClass('hide');
          }
          if (!response.data.updating_news_now && !$('#updating_news_now').hasClass('hide')) {
            $('#updating_news_now').addClass('hide');
          }
          if (is_market_open && $('#is_market_open_color').hasClass("text-danger")) {
              $("#ticker_quote_info").show();
              $("#ticker_eod_info").hide();
              $('#is_market_open_color').animate({opacity: 0}, 400, function() { $('#is_market_open_color').removeClass("text-danger").addClass("text-success").animate({opacity: 1}, 400) });
              $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text("TRADING").animate({opacity: 1}, 400) });
          } else if (!is_market_open && $('#is_market_open_color').hasClass("text-success")) {
              $("#ticker_quote_info").hide();
              $("#ticker_eod_info").show();
              $('#is_market_open_color').animate({opacity: 0}, 400, function() { ($('#is_market_open_color').removeClass("text-success").addClass("text-danger").animate({opacity: 1}, 400)) });
              $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text("CLOSED").animate({opacity: 1}, 400) });
          }
      },
      complete: function() {
          setTimeout(function() { $('#auto_refresh_working').addClass('hide'); }, 1000);
          if (update_count > 0) {
              update_count--;
              setTimeout(function() { update_quotes(); }, quote_refresh * 1000);
          }
      }
  });
}

$(document).ready(function() {
    setTimeout(function() {
        update_quote();
    }, quote_refresh * 1000);

    $('.inline-article').find('.CMS__Security').each(function (index) {
        symbol = this.text();
        this.innerHTML = '<a href="/view/' + symbol + '">' + symbol + '</a>';
    });

    if (is_market_open) {
        $("#ticker_quote_info").show();
    } else {
        $("#ticker_eod_info").show();
    }

    $('#auto_refresh').on('click', function() {
        if ($('#auto_refresh > i').hasClass("fa-spin")) {
            $('#auto_refresh > i').animate({opacity: 0}, 400, function() { ($('#auto_refresh > i').removeClass("fa-spin").addClass("fa-pause-circle").animate({opacity: 1}, 400)) });
            $('#auto_refresh_time').animate({opacity: 0}, 400, function() { ($('#auto_refresh_time').text("paused").animate({opacity: 1}, 400)) });
            update_count = 0;
        } else {
            $('#auto_refresh > i').animate({opacity: 0}, 400, function() { ($('#auto_refresh > i').removeClass("fa-pause-circle").addClass("fa-spin").animate({opacity: 1}, 400)) });
            $('#auto_refresh_time').animate({opacity: 0}, 400, function() { ($('#auto_refresh_time').text("20 sec").animate({opacity: 1}, 400)) });
            update_count = 180;
            update_quote();
        }
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
