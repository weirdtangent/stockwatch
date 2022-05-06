$(document).ready(function() {
    if (is_market_open) {
        $('#auto_refresh_time').text('20 sec');
        setTimeout(function() { quoteRefresh(); }, quote_refresh * 1000);
    } else { // 15 times slower if market isn't even open (so every 300 sec instead of 20 sec)
        $('#auto_refresh_time').text('5 min');
        setTimeout(function() { quoteRefresh(); }, quote_refresh * 1000 * 15);
    }

    if (is_market_open) {
        $("#ticker_quote_info").show();
        $("#auto_refresh").show();
    } else {
        $("#ticker_eod_info").show();
    }

    $('#auto_refresh_link').on('click', function() {
        if ($('#auto_refresh').hasClass("fa-spin")) {
            $('#auto_refresh').animate({opacity: 0}, 400, function() { ($('#auto_refresh').removeClass("fa-spin").removeClass("fa-sync").addClass("fa-pause-circle").animate({opacity: 1}, 400)) });
            $('#auto_refresh_time').animate({opacity: 0}, 400, function() { ($('#auto_refresh_time').text("paused").animate({opacity: 1}, 400)) });
            update_count = 0;
        } else {
            $('#auto_refresh').animate({opacity: 0}, 400, function() { ($('#auto_refresh').removeClass("fa-pause-circle").addClass("fa-sync").addClass("fa-spin").animate({opacity: 1}, 400)) });
            $('#auto_refresh_time').animate({opacity: 0}, 400, function() { ($('#auto_refresh_time').text(quote_refresh + " sec").animate({opacity: 1}, 400)) });
            update_count = 180;
            quoteRefresh();
        }
    })

    $('.btn-close').on('click', function() {
        var symbol = $(this).data('symbol')
        var response = $.ajax({
            type: 'GET',
            url: '/api/v1/recents?remove=' + symbol,
            async: false,
            success: function(response) {
                if (response.success) {
                    $("#"+symbol+"_card").animate({opacity:0}, 800, function() { $(this).hide(); });
                }
            }
        });
    })
});