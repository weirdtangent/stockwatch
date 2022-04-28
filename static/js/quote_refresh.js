var scripts = document.getElementsByTagName('script');
var lastScript = scripts[scripts.length-1];
var scriptName = lastScript;

var symbols = scriptName.getAttribute('data-symbols');
var is_market_open = scriptName.getAttribute('data-is-market-open') === "true";
var quote_refresh = scriptName.getAttribute('data-quote-refresh');

// every 20 sec for 1 hour = 180 refreshes
var update_count = 180

function quoteRefresh() {
    if (update_count <= 0) {
        return;
    }
    $('#auto_refresh_working').removeClass('hide');
    var response = $.ajax({
        type: 'GET',
        url: '/api/v1/quotes?symbols=' + symbols,
        async: true,
        success: function(response) {
            is_market_open = response.data.is_market_open === true;
            symbols.split(",").forEach(function(item) {
                if (item == "") { return; }
                symbol = item;
                ['quote_shareprice', 'quote_ask', 'quote_asksize', 'quote_bid', 'quote_bidsize', 'quote_asof', 'quote_change', 'quote_change_pct'].forEach(function(item) {
                    phaseChange(response, symbol, item)
                });

                if (response.data.symbol+":quote_dailymove" === 'down' && !$('#'+symbol+'_quote_dailymove').hasClass("fa-arrow-down")) {
                    $('#'+symbol+'_quote_dailymove_text').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove_text').removeClass("text-success").addClass("text-danger").animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_quote_dailymove').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove').removeClass("fa-arrow-up text-success").addClass("fa-arrow-down text-danger").animate({opacity: 1}, 400)
                    });
                } else if (response.data.symbol+":quote_dailymove" === 'up' && !$('#'+symbol+'_quote_dailymove').hasClass("fa-up-down")) {
                    $('#'+symbol+'_quote_dailymove_text').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove_text').removeClass("text-danger").addClass("text-success").animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_quote_dailymove').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove').removeClass("fa-arrow-down text-danger").addClass("fa-arrow-up text-success").animate({opacity: 1}, 400)
                    });
                } else if (response.data.symbol+':quote_dailymove' === 'unchanged' && !$('#'+symbol+'_quote_dailymove').hasClass("fa-equals")) {
                    $('#'+symbol+'_quote_dailymove_text').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove_text').removeClass("text-danger").removeClass("text-success").animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_quote_dailymove').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'quote_dailymove').removeClass("fa-arrow-down text-danger").removeClass("fa-arrow-up text-success").addClass("fa-equals").animate({opacity: 1}, 400)
                    });
                }

                phaseChange(response, symbol, 'last_checked_news')
                if (response.data.symbol+':updating_news_now'=='true' && $('#'+symbol+'_updating_news_now').hasClass('hide')) {
                    $('#'+symbol+'_updating_news_now').removeClass('hide');
                } else if (response.data.symbol+':updating_news_now'=='false' && !$('#'+symbol+'_updating_news_now').hasClass('hide')) {
                    $('#'+symbol+'_updating_news_now').addClass('hide');
                }
            });

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
                setTimeout(function() { quoteRefresh(); }, quote_refresh * 1000);
            }
        }
    });
}

function phaseChange(response, symbol, item) {
    var itemId = `#${symbol}_${item}`
    var dataId = `${symbol}:${item}`
    var oldValue = $(itemId).text()
    if (typeof response[`data`][dataId] === 'undefined') { return }
    var newValue = response[`data`][dataId]

    if (oldValue != newValue) {
        // console.log(`going to set '${item}' for '${symbol}' from '${oldValue}' to '${newValue}'`);
        $(itemId).animate({opacity: 0}, 400, function() {
            $(itemId).text(newValue).animate({opacity: 1}, 400);
        });
    } else {
        // console.log(`leaving '${item}' for '${symbol}' alone since it hasn't changed`);
    }
}