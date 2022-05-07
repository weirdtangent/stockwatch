var chart = $('#chartCall').data('chart');
var symbol = $('#chartCall').data('symbol');
var nonce = $('#chartCall').data('nonce');
var timespan = $('#chartCall').data('timespan');

function loadChart(chart, symbol, timespan) {
    var response = $.ajax({
        type: 'GET',
        headers: { 'X-Nonce': nonce },
        url: '/api/v1/chart?chart=' + chart + '&symbol=' + symbol + '&timespan=' +timespan,
        async: true,
        success: function(response) {
            if (response.success == true) {
                $('#tickerChart').html(response.data['chartHTML']);
            }
        }
    });
}