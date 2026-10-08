// Highlights every Compare badge sharing the hovered, tapped or focused
// badge's data-match key within the same table row. Progressive
// enhancement: the table renders fully without this script.
(function () {
  var BADGE = '.cmp-tag[data-match]';
  var ACTIVE = 'cmp-tag--match';

  function clear() {
    document.querySelectorAll('.' + ACTIVE).forEach(function (el) {
      el.classList.remove(ACTIVE);
    });
  }

  function badgeOf(target) {
    return target instanceof Element ? target.closest(BADGE) : null;
  }

  function highlight(badge) {
    clear();
    var row = badge.closest('tr');
    if (!row) {
      return;
    }
    row.querySelectorAll(BADGE).forEach(function (other) {
      if (other.getAttribute('data-match') === badge.getAttribute('data-match')) {
        other.classList.add(ACTIVE);
      }
    });
  }

  function onEnter(event) {
    var badge = badgeOf(event.target);
    if (badge) {
      highlight(badge);
    }
  }

  function onLeave(event) {
    if (badgeOf(event.target)) {
      clear();
    }
  }

  document.addEventListener('mouseover', onEnter);
  document.addEventListener('focusin', onEnter);
  document.addEventListener('mouseout', onLeave);
  document.addEventListener('focusout', onLeave);
  document.addEventListener('click', function (event) {
    var badge = badgeOf(event.target);
    if (badge) {
      highlight(badge);
    } else {
      clear();
    }
  });
})();
