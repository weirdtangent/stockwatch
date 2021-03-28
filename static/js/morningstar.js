$(document).ready(function() { 
  $('.CMS__Security').each(function (index) {
    symbol = $(this).text();
    this.innerHTML = '<a href="/view/' + symbol + '">' + symbol + '</a>';
  });
});
