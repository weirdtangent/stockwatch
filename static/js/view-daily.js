var showing = 'symbolLine';

$(document).ready(function() {
    loadChart("symbolLine", symbol, timespan)

    setTimeout(function() {
        quoteRefresh();
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

    $('input[name=pickChart]').on('change', function() {
        chart = $('input[name=pickChart]:checked').attr('id');
        timespan = $('input[name=pickTimespan]:checked').data('timespan')
        $('#tickerChart').fadeOut('fast', function() {
            loadChart(chart, symbol, timespan);
            $('#tickerChart').fadeIn('fast');
        });
        showing = chart;
    });


    $('input[name=pickTimespan]').on('change', function() {
        chart = $('input[name=pickChart]:checked').attr('id');
        timespan = $('input[name=pickTimespan]:checked').data('timespan')
        $('#tickerChart').fadeOut('fast', function() {
            loadChart(chart, symbol, timespan);
            $('#tickerChart').fadeIn('fast');
        });
    });

    Date.prototype.toDateInputValue = (function() {
        var local = new Date(this);
        local.setMinutes(this.getMinutes() - this.getTimezoneOffset());
        return local.toJSON().slice(0,10);
    });

    $('#PurchaseDate').val(new Date().toDateInputValue());

});
