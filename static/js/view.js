$(function() {
  // line vs kline chart
  $('#toggle > button').click(function() {
    var count = $(this).index();

    $('#lineChart').toggle( count === 0 );
    $('#klineChart').toggle( count === 1 );
  });
});
