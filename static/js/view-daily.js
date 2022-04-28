var showing = 'tickerChart1';

$(document).ready(function() {
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
