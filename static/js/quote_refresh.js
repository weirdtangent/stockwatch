var scripts = document.getElementsByTagName('script');
var lastScript = scripts[scripts.length-1];
var scriptName = lastScript;

var symbols = scriptName.getAttribute('data-symbols');
var is_market_open = scriptName.getAttribute('data-is-market-open') === 'true';

var quote_refresh = 20; // scriptName.getAttribute('data-quote-refresh');
var update_count = 180; // every 20 sec for 1 hour = 180 refreshes

function quoteRefresh() {
    if (update_count <= 0 || symbols == '') {
        return;
    }
    $('#auto_refresh_working').removeClass('hide');
    while (symbols.charAt(symbols.length-1) == ',') {
        symbols = symbols.substring(0, symbols.length-1);
    }
    var response = $.ajax({
        type: 'GET',
        url: '/api/v1/quotes?symbols=' + symbols,
        async: true,
        success: function(response) {
            is_market_open = response.data.is_market_open;
            symbols.split(',').forEach(function(item) {
                if (item == '') { return; }
                symbol = item;
                ['price', 'ask', 'asksize', 'bid', 'bidsize', 'asof', 'change_amt', 'change_pct'].forEach(function(item) {
                    phaseChangeSymbol(response, symbol, item)
                });

                if (response.data.symbol+':change_dir' === 'down' && !$('#'+symbol+'_change_indicator').hasClass('fa-arrow-down')) {
                    $('#'+symbol+'_change_color').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_color').removeClass('text-success').addClass('text-danger').animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_change_indicator').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_indicator').removeClass('fa-arrow-up text-success').addClass('fa-arrow-down text-danger').animate({opacity: 1}, 400)
                    });
                } else if (response.data.symbol+':change_dir' === 'up' && !$('#'+symbol+'_change_indicator').hasClass('fa-up-down')) {
                    $('#'+symbol+'_change_color').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_color').removeClass('text-danger').addClass('text-success').animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_change_indicator').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_indicatgor').removeClass('fa-arrow-down text-danger').addClass('fa-arrow-up text-success').animate({opacity: 1}, 400)
                    });
                } else if (response.data.symbol+':change_dir' === 'unchanged' && !$('#'+symbol+'_change_indicator').hasClass('fa-equals')) {
                    $('#'+symbol+'_change_color').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_color').removeClass('text-danger').removeClass('text-success').animate({opacity: 1}, 400)
                    });
                    $('#'+symbol+'_change_indicator').animate({opacity: 0}, 400, function() {
                        $('#'+symbol+'_change_indicator').removeClass('fa-arrow-down text-danger').removeClass('fa-arrow-up text-success').addClass('fa-equals').animate({opacity: 1}, 400)
                    });
                }

                phaseChangeSymbol(response, symbol, 'last_checked_since')
                if (response.data.symbol+':updating_now' == true) {
                    $('#'+symbol+'_updating_news_now').removeClass('hide');
                } else if (response.data.symbol+':updating_now' == false) {
                    $('#'+symbol+'_updating_news_now').addClass('hide');
                }
            });

            phaseChange(response, 'last_checked_since')
            if (response.data.updating_news_now=='true' && $('#updating_news_now').hasClass('hide')) {
                $('#updating_news_now').removeClass('hide');
            } else if (response.data.updating_news_now=='false' && !$('#updating_news_now').hasClass('hide')) {
                $('#updating_news_now').addClass('hide');
            }

            if (is_market_open && $('#is_market_open_color').hasClass('text-danger')) {
                $('#ticker_quote_info').show();
                $('#ticker_eod_info').hide();
                $('#is_market_open_color').animate({opacity: 0}, 400, function() { $('#is_market_open_color').removeClass('text-danger').addClass('text-success').animate({opacity: 1}, 400) });
                $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text('TRADING').animate({opacity: 1}, 400) });
            } else if (!is_market_open && $('#is_market_open_color').hasClass('text-success')) {
                $('#ticker_quote_info').hide();
                $('#ticker_eod_info').show();
                $('#is_market_open_color').animate({opacity: 0}, 400, function() { ($('#is_market_open_color').removeClass('text-success').addClass('text-danger').animate({opacity: 1}, 400)) });
                $('#is_market_open').animate({opacity: 0}, 400, function() { $('#is_market_open').text('CLOSED').animate({opacity: 1}, 400) });
            }
        },
        complete: function() {
            setTimeout(function() { $('#auto_refresh_working').addClass('hide'); }, 1000);
            if (update_count > 1) {
                update_count--;
                if (is_market_open) {
                    $('#auto_refresh_time').text('20 sec');
                    setTimeout(function() { quoteRefresh(); }, quote_refresh * 1000);
                } else { // 15 times slower if market isn't even open (so every 300 sec instead of 20 sec)
                    $('#auto_refresh_time').text('5 min');
                    setTimeout(function() { quoteRefresh(); }, quote_refresh * 1000 * 15);
                }
            } else {
                $('#auto_refresh').animate({opacity: 0}, 400, function() {
                    $('#auto_refresh').removeClass('fa-spin').removeClass('fa-sync').addClass('fa-pause-circle').animate({opacity: 1}, 400);
                });
                $('#auto_refresh_time').text('paused');
            }
        }
    });
}

function phaseChange(response, item) {
    var itemId = `#${item}`
    var dataId = `${item}`
    var oldValue = $(itemId).text()
    if (typeof response[`data`][dataId] === 'undefined') { return }
    var newValue = response[`data`][dataId]

    if (oldValue != newValue) {
        $(itemId).animate({opacity: 0}, 400, function() {
            $(itemId).text(newValue).animate({opacity: 1}, 400);
        });
    }
}

function phaseChangeSymbol(response, symbol, item) {
    var itemId = `#${symbol}_${item}`
    var dataId = `${symbol}:${item}`
    var oldValue = $(itemId).text()
    if (typeof response[`data`][dataId] === 'undefined') { return }
    var newValue = response[`data`][dataId]

    if (oldValue != newValue) {
        $(itemId).animate({opacity: 0}, 400, function() {
            $(itemId).text(newValue).animate({opacity: 1}, 400);
        });
    }
}