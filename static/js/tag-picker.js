// Tag picker component — searchable multi-select with chips.
// Usage:
//   <div class="tag-picker" data-name="guest_ids"
//        data-items='[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]'
//        data-selected='[1]'></div>
//
// Renders: chips for selected items, search input, dropdown results.
// Submits hidden <input name="guest_ids" value="1"> per selected item.

(function () {
  function createRemoveIcon() {
    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'h-3 w-3');
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('viewBox', '0 0 24 24');
    var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');
    path.setAttribute('stroke-width', '2');
    path.setAttribute('d', 'M6 18L18 6M6 6l12 12');
    svg.appendChild(path);
    return svg;
  }

  document.querySelectorAll('.tag-picker').forEach(initPicker);

  function initPicker(container) {
    var fieldName = container.dataset.name;
    var items = JSON.parse(container.dataset.items || '[]') || [];
    var selected = new Set((JSON.parse(container.dataset.selected || '[]') || []).map(Number));
    var activeIndex = -1;
    var pickerId = 'tag-picker-' + fieldName + '-' + Math.random().toString(36).substr(2, 6);
    var listboxId = pickerId + '-listbox';

    // Build the DOM
    while (container.firstChild) container.removeChild(container.firstChild);

    var chipsWrap = document.createElement('div');
    chipsWrap.className = 'flex flex-wrap gap-1.5 mb-2';
    chipsWrap.style.minHeight = '0';
    container.appendChild(chipsWrap);

    var inputWrap = document.createElement('div');
    inputWrap.className = 'relative';
    container.appendChild(inputWrap);

    var input = document.createElement('input');
    input.type = 'text';
    input.placeholder = 'Search\u2026';
    input.autocomplete = 'off';
    input.className = 'block w-full rounded-md border border-gray-300 dark:border-gray-600 px-3 py-2 shadow-sm ' +
      'focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:bg-gray-700 dark:text-gray-100 text-sm';
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-controls', listboxId);
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-label', 'Search ' + fieldName.replace(/_/g, ' '));
    inputWrap.appendChild(input);

    var dropdown = document.createElement('ul');
    dropdown.id = listboxId;
    dropdown.setAttribute('role', 'listbox');
    dropdown.className = 'absolute z-10 mt-1 max-h-48 w-full overflow-auto rounded-md bg-white dark:bg-gray-700 ' +
      'shadow-lg ring-1 ring-black/5 dark:ring-white/10 hidden';
    inputWrap.appendChild(dropdown);

    function getVisibleMatches() {
      var query = input.value.toLowerCase().trim();
      return items.filter(function (item) {
        return !selected.has(item.id) && item.name.toLowerCase().indexOf(query) !== -1;
      });
    }

    // Render functions
    function renderChips() {
      while (chipsWrap.firstChild) chipsWrap.removeChild(chipsWrap.firstChild);
      // Also clear old hidden inputs
      container.querySelectorAll('input[type="hidden"]').forEach(function (el) { el.remove(); });

      items.forEach(function (item) {
        if (!selected.has(item.id)) return;

        // Hidden form input
        var hidden = document.createElement('input');
        hidden.type = 'hidden';
        hidden.name = fieldName;
        hidden.value = item.id;
        container.appendChild(hidden);

        // Chip
        var chip = document.createElement('span');
        chip.className = 'inline-flex items-center gap-1 rounded-full bg-indigo-50 dark:bg-indigo-900/30 ' +
          'px-2.5 py-1 text-sm font-medium text-indigo-700 dark:text-indigo-400';
        chip.textContent = item.name;

        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'ml-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full ' +
          'text-indigo-400 hover:bg-indigo-200 dark:hover:bg-indigo-800 hover:text-indigo-600 dark:hover:text-indigo-300';
        btn.setAttribute('aria-label', 'Remove ' + item.name);
        btn.appendChild(createRemoveIcon());
        btn.addEventListener('click', function () {
          selected.delete(item.id);
          renderChips();
          renderDropdown();
        });
        chip.appendChild(btn);
        chipsWrap.appendChild(chip);
      });
    }

    function showDropdown() {
      dropdown.classList.remove('hidden');
      input.setAttribute('aria-expanded', 'true');
    }

    function hideDropdown() {
      dropdown.classList.add('hidden');
      input.setAttribute('aria-expanded', 'false');
      activeIndex = -1;
      input.removeAttribute('aria-activedescendant');
    }

    function highlightItem(index) {
      var optionEls = dropdown.querySelectorAll('[role="option"]');
      optionEls.forEach(function (li, i) {
        if (i === index) {
          li.classList.add('bg-indigo-50', 'dark:bg-indigo-900/30', 'text-indigo-700', 'dark:text-indigo-400');
          li.setAttribute('aria-selected', 'true');
          input.setAttribute('aria-activedescendant', li.id);
          li.scrollIntoView({ block: 'nearest' });
        } else {
          li.classList.remove('bg-indigo-50', 'dark:bg-indigo-900/30', 'text-indigo-700', 'dark:text-indigo-400');
          li.removeAttribute('aria-selected');
        }
      });
    }

    function selectItem(item) {
      selected.add(item.id);
      input.value = '';
      activeIndex = -1;
      renderChips();
      renderDropdown();
    }

    function renderDropdown() {
      var query = input.value.toLowerCase().trim();
      while (dropdown.firstChild) dropdown.removeChild(dropdown.firstChild);
      activeIndex = -1;
      input.removeAttribute('aria-activedescendant');

      var matches = getVisibleMatches();

      if (matches.length === 0 || query === '') {
        hideDropdown();
        return;
      }

      matches.forEach(function (item, i) {
        var li = document.createElement('li');
        li.id = pickerId + '-option-' + i;
        li.setAttribute('role', 'option');
        li.className = 'cursor-pointer px-3 py-2 text-sm text-gray-700 dark:text-gray-200 ' +
          'hover:bg-indigo-50 dark:hover:bg-indigo-900/30 hover:text-indigo-700 dark:hover:text-indigo-400';
        li.textContent = item.name;
        li.addEventListener('mousedown', function (e) {
          e.preventDefault(); // prevent input blur
          selectItem(item);
        });
        dropdown.appendChild(li);
      });

      showDropdown();
    }

    input.addEventListener('input', renderDropdown);
    input.addEventListener('focus', function () {
      if (input.value.trim() !== '') renderDropdown();
    });
    input.addEventListener('blur', function () {
      // Delay to allow mousedown on dropdown items
      setTimeout(function () { hideDropdown(); }, 150);
    });

    input.addEventListener('keydown', function (e) {
      var matches = getVisibleMatches();
      var isOpen = !dropdown.classList.contains('hidden');

      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (!isOpen && input.value.trim() !== '') {
          renderDropdown();
          matches = getVisibleMatches();
          isOpen = !dropdown.classList.contains('hidden');
        }
        if (isOpen && matches.length > 0) {
          activeIndex = (activeIndex + 1) % matches.length;
          highlightItem(activeIndex);
        }
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (isOpen && matches.length > 0) {
          activeIndex = activeIndex <= 0 ? matches.length - 1 : activeIndex - 1;
          highlightItem(activeIndex);
        }
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (isOpen && activeIndex >= 0 && activeIndex < matches.length) {
          selectItem(matches[activeIndex]);
        }
      } else if (e.key === 'Escape') {
        if (isOpen) {
          e.preventDefault();
          hideDropdown();
        }
      }
    });

    // Initial render
    renderChips();
  }
})();
