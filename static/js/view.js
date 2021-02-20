$(function() {
  // line vs kline chart
  $('#toggle > button').click(function() {
    var count = $(this).index();

    if (count === 0 ) {
      $('#klineChart').fadeOut('fast', function() {
        $('#lineChart').fadeIn('fast');
      });
    } else if (count === 1) {
      $('#lineChart').fadeOut('fast', function() {
        $('#klineChart').fadeIn('fast');
      });
    } else if (count === 2) {
      $('#lineChart').fadeOut('fast');
      $('#klineChart').fadeOut('fast');
    }
  });
});
