
$(document).ready(function() {
  var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'))
  var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
      return new bootstrap.Tooltip(tooltipTriggerEl)
  })

  function resizedW() {
      const vw = Math.max(document.documentElement.clientWidth || 0, window.innerWidth || 0)
      if (vw < 1634) {
        alert(`Sorry I demand such a wide screen, but anything < 1634 pixels wide just isn't any fun on the site and you are currently at ${vw}`)
      }
  }

  resizedW();

  var recheckVW;
  window.onresize = function() {
    clearTimeout(recheckVW);
    recheckVW = setTimeout(resizedW, 100);
  };
});
