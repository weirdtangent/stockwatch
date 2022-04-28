$(document).ready(function() {
    setTimeout(function() {
        quoteRefresh();
    }, quote_refresh * 1000);

    if (is_market_open) {
        $("#ticker_quote_info").show();
        $("#auto_refresh").show();
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
            update_quotes();
        }
    })
});