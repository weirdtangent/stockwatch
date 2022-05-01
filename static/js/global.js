
$(document).ready(function() {
  var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'))
  var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
      return new bootstrap.Tooltip(tooltipTriggerEl)
  })

  var warned_too_small = false
  function resizedW() {
      const vw = Math.max(document.documentElement.clientWidth || 0, window.innerWidth || 0)
      if (vw < 1634 && !warned_too_small) {
        warned_too_small = true
        console.log(`Sorry I demand such a wide screen, but anything < 1634 pixels wide just isn't any fun on the site and you are currently at ${vw}`)
      }
  }

  resizedW();

  var recheckVW;
  window.onresize = function() {
    clearTimeout(recheckVW);
    recheckVW = setTimeout(resizedW, 100);
  };

  $('.lock-button').on('click', function() {
    var symbol = $(this).data('symbol')
    if ($(this).hasClass("fa-lock")) {
      var response = $.ajax({
          type: 'GET',
          url: '/api/v1/recents?unlock=' + symbol,
          async: false,
          success: function(response) {
              if (response.success) {
                  $("#"+symbol+"_lock_button").removeClass("fa-lock").addClass("fa-lock-open");
                  $("#"+symbol+"_lock_badge").removeClass("bg-success").addClass("bg-warning");
                  $("#"+symbol+"_close_button").removeClass("disabled")
              }
          }
      });
    } else if ($(this).hasClass("fa-lock-open")) {
      var response = $.ajax({
          type: 'GET',
          url: '/api/v1/recents?lock=' + symbol,
          async: false,
          success: function(response) {
              if (response.success) {
                  $("#"+symbol+"_lock_button").removeClass("fa-lock-open").addClass("fa-lock");
                  $("#"+symbol+"_lock_badge").removeClass("bg-warning").addClass("bg-success");
                  $("#"+symbol+"_close_button").addClass("disabled")
              }
          }
      });
    }
  });
});
