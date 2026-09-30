// Local graph enhancement.
//
// The server draws a static SVG of the page's one-hop graph, which works on its
// own and whose nodes are real links. This script replaces that drawing with an
// interactive one: small dots held by springs, where hovering shows the page's
// full title and dragging pulls the web around. It reads the same nodes and
// edges from the JSON the page embeds, and if anything goes wrong the static
// SVG stays: the drawing is the feature and this is the upgrade.
(function () {
  var dataEl = document.querySelector("script.graph-data");
  if (!dataEl || !window.requestAnimationFrame) {
    return;
  }
  var figure = dataEl.closest(".graph");
  var staticSvg = figure && figure.querySelector("svg.graph-svg");
  if (!staticSvg) {
    return;
  }

  var data;
  try {
    data = JSON.parse(dataEl.textContent);
  } catch (e) {
    return;
  }

  // Two shapes reach this script: a page's local graph, which is a centre and
  // its neighbours, and the whole-KB graph, which is nodes and edges outright.
  // Both become the same two lists. Only the whole-KB graph sizes its nodes;
  // the local graph draws them uniform.
  var sized = !!data.nodes;
  var nodes, edges;
  if (data.nodes) {
    nodes = data.nodes.map(function (n) {
      return { title: n.title, url: n.url, self: !!n.self, backlinks: n.backlinks || 0, degree: n.degree || 0, color: n.color || 0 };
    });
    edges = data.edges || [];
  } else {
    nodes = [{ title: data.center.title, url: data.center.url, self: true, backlinks: data.center.backlinks || 0 }];
    edges = [];
    (data.backlinks || []).forEach(function (n) {
      edges.push([0, nodes.length]);
      nodes.push({ title: n.title, url: n.url, backlinks: n.backlinks || 0 });
    });
    (data.links || []).forEach(function (n) {
      edges.push([0, nodes.length]);
      nodes.push({ title: n.title, url: n.url, backlinks: n.backlinks || 0 });
    });
  }
  if (nodes.length < 2) {
    return;
  }

  var NS = "http://www.w3.org/2000/svg";
  var hasSelf = nodes.some(function (n) { return n.self; });
  var W = hasSelf ? 640 : 760;
  var H = hasSelf ? Math.max(260, 150 + nodes.length * 26) : 760;

  // In the whole-KB graph a node's size is its degree -- backlinks plus
  // outgoing links -- square-rooted into a fixed range so one hub cannot dwarf
  // the rest. The local graph is left uniform: its shape already says what
  // connects to what, so size there would say nothing new.
  var MIN_R = 5;
  var MAX_R = 18;
  var top = Math.max(1, data.max_degree || 1);
  function radius(n) {
    if (!sized) {
      return n.self ? 12 : 9;
    }
    return MIN_R + (MAX_R - MIN_R) * Math.sqrt(Math.max(0, n.degree || 0) / top);
  }

  // Springs and charge. The numbers are a balance rather than a derivation:
  // enough repulsion that dots do not overlap, enough spring that connected
  // dots stay near each other, and damping so the web settles instead of
  // humming forever.
  var REPULSION = 9000;
  var SPRING = 0.02;
  var REST = 92;
  var DAMPING = 0.88;
  var CENTER_PULL = 0.004;
  var MAX_SPEED = 16;

  // A little irregularity, so the web does not settle into a symmetric ring:
  // every spring gets its own rest length, and every node starts a little off
  // where it was seeded. Both are drawn once at start-up rather than per frame,
  // so the layout can still come to rest instead of twitching forever.
  var edgeRest = edges.map(function () {
    return REST * (0.8 + Math.random() * 0.4);
  });
  var JITTER = 0.12;

  function clamp(v, lo, hi) {
    return v < lo ? lo : v > hi ? hi : v;
  }

  var selfIndex = -1;
  for (i = 0; i < nodes.length; i++) {
    nodes[i].r = radius(nodes[i]);
    if (nodes[i].self && selfIndex < 0) {
      selfIndex = i;
    }
  }
  var around = 0;
  var others = nodes.length - (selfIndex < 0 ? 0 : 1);
  for (i = 0; i < nodes.length; i++) {
    if (i === selfIndex) {
      nodes[i].x = W / 2;
      nodes[i].y = H / 2;
      continue;
    }
    var angle = (around / Math.max(1, others)) * Math.PI * 2;
    around++;
    nodes[i].x = W / 2 + Math.cos(angle) * W * 0.32 + (Math.random() - 0.5) * W * JITTER;
    nodes[i].y = H / 2 + Math.sin(angle) * H * 0.34 + (Math.random() - 0.5) * H * JITTER;
  }
  nodes.forEach(function (n) { n.vx = 0; n.vy = 0; });

  // step advances the web one frame and returns its kinetic energy, so the
  // loop can stop once it has settled.
  function step() {
    var i, j, dx, dy, d, f, ux, uy, n;

    for (i = 0; i < nodes.length; i++) {
      nodes[i].fx = 0;
      nodes[i].fy = 0;
    }
    for (i = 0; i < nodes.length; i++) {
      for (j = i + 1; j < nodes.length; j++) {
        dx = nodes[i].x - nodes[j].x;
        dy = nodes[i].y - nodes[j].y;
        d = Math.sqrt(dx * dx + dy * dy) || 0.01;
        f = REPULSION / (d * d);
        ux = dx / d;
        uy = dy / d;
        nodes[i].fx += ux * f;
        nodes[i].fy += uy * f;
        nodes[j].fx -= ux * f;
        nodes[j].fy -= uy * f;
      }
    }
    for (i = 0; i < edges.length; i++) {
      var a = nodes[edges[i][0]];
      var b = nodes[edges[i][1]];
      dx = a.x - b.x;
      dy = a.y - b.y;
      d = Math.sqrt(dx * dx + dy * dy) || 0.01;
      f = (d - edgeRest[i]) * SPRING;
      ux = dx / d;
      uy = dy / d;
      a.fx -= ux * f;
      a.fy -= uy * f;
      b.fx += ux * f;
      b.fy += uy * f;
    }

    var energy = 0;
    for (i = 0; i < nodes.length; i++) {
      n = nodes[i];
      if (n.fixed) {
        n.vx = 0;
        n.vy = 0;
        continue;
      }
      n.fx += (W / 2 - n.x) * CENTER_PULL;
      n.fy += (H / 2 - n.y) * CENTER_PULL;
      n.vx = (n.vx + n.fx) * DAMPING;
      n.vy = (n.vy + n.fy) * DAMPING;
      var speed = Math.sqrt(n.vx * n.vx + n.vy * n.vy);
      if (speed > MAX_SPEED) {
        n.vx = (n.vx / speed) * MAX_SPEED;
        n.vy = (n.vy / speed) * MAX_SPEED;
      }
      n.x = clamp(n.x + n.vx, n.r + 2, W - n.r - 2);
      n.y = clamp(n.y + n.vy, n.r + 2, H - n.r - 2);
      energy += n.vx * n.vx + n.vy * n.vy;
    }
    return energy;
  }

  var svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", "graph-svg");
  svg.setAttribute("viewBox", "0 0 " + W + " " + H);
  svg.setAttribute("role", "group");
  svg.setAttribute("aria-label", "Local graph");
  var title = document.createElementNS(NS, "title");
  title.textContent = "Local graph";
  svg.appendChild(title);

  var edgeEls = edges.map(function () {
    var line = document.createElementNS(NS, "line");
    line.setAttribute("class", "graph-edge");
    svg.appendChild(line);
    return line;
  });

  var nodeEls = nodes.map(function (n, index) {
    var wrap = n.self
      ? document.createElementNS(NS, "g")
      : document.createElementNS(NS, "a");
    var cls = n.self ? "graph-node graph-self" : "graph-node";
    if (sized) {
      cls += " graph-color-" + (n.color || 0);
    }
    wrap.setAttribute("class", cls);
    if (!n.self) {
      wrap.setAttribute("href", n.url);
    }
    wrap.setAttribute("data-i", index);

    var dot = document.createElementNS(NS, "circle");
    dot.setAttribute("r", n.r);
    wrap.appendChild(dot);
    svg.appendChild(wrap);
    return wrap;
  });

  // The hover title: one label, moved to whichever dot is under the pointer, so
  // the dots can stay dots.
  var tip = document.createElementNS(NS, "g");
  tip.setAttribute("class", "graph-tip");
  var tipRect = document.createElementNS(NS, "rect");
  tipRect.setAttribute("rx", 4);
  var tipText = document.createElementNS(NS, "text");
  tipText.setAttribute("x", 0);
  tipText.setAttribute("y", 0);
  tip.appendChild(tipRect);
  tip.appendChild(tipText);
  tip.style.display = "none";
  svg.appendChild(tip);

  var hovered = -1;

  function tipLabel(n) {
    var c = n.backlinks || 0;
    var text = c === 0 ? "no backlinks" : c + " backlink" + (c === 1 ? "" : "s");
    return n.title + " \u2014 " + text;
  }

  function positionTip() {
    var n = nodes[hovered];
    tipText.textContent = tipLabel(n);
    tip.style.display = "";
    var box = tipText.getBBox();
    tipRect.setAttribute("x", box.x - 7);
    tipRect.setAttribute("y", box.y - 5);
    tipRect.setAttribute("width", box.width + 14);
    tipRect.setAttribute("height", box.height + 10);

    var x = n.x + n.r + 8;
    var y = n.y - n.r - 8;
    if (x + box.width + 16 > W) {
      x = n.x - n.r - 8 - (box.width + 14);
    }
    if (y < 2) {
      y = n.y + n.r + 8 + box.height + 4;
    }
    tip.setAttribute("transform", "translate(" + x + "," + y + ")");
  }

  function draw() {
    edgeEls.forEach(function (line, k) {
      line.setAttribute("x1", nodes[edges[k][0]].x);
      line.setAttribute("y1", nodes[edges[k][0]].y);
      line.setAttribute("x2", nodes[edges[k][1]].x);
      line.setAttribute("y2", nodes[edges[k][1]].y);
    });
    nodeEls.forEach(function (el, k) {
      el.setAttribute("transform", "translate(" + nodes[k].x + "," + nodes[k].y + ")");
    });
  }

  var running = false;
  var drag = -1;
  var moved = false;

  function frame() {
    var energy = 0;
    for (var s = 0; s < 2; s++) {
      energy = step();
    }
    draw();
    if (hovered >= 0) {
      positionTip();
    }
    if (energy < 0.05 && drag < 0) {
      running = false;
      return;
    }
    requestAnimationFrame(frame);
  }

  function wake() {
    if (!running) {
      running = true;
      requestAnimationFrame(frame);
    }
  }

  nodeEls.forEach(function (el, index) {
    el.addEventListener("pointerdown", function (ev) {
      drag = index;
      moved = false;
      nodes[index].fixed = true;
      if (el.setPointerCapture) {
        el.setPointerCapture(ev.pointerId);
      }
      wake();
    });
    el.addEventListener("pointerenter", function () {
      hovered = index;
      positionTip();
    });
    el.addEventListener("pointerleave", function () {
      if (hovered === index) {
        hovered = -1;
        tip.style.display = "none";
      }
    });
    el.addEventListener("click", function (ev) {
      // A drag ends with a click on the dot that was dragged; do not follow it.
      if (moved) {
        ev.preventDefault();
      }
    });
  });

  svg.addEventListener("pointermove", function (ev) {
    if (drag < 0) {
      return;
    }
    moved = true;
    var box = svg.getBoundingClientRect();
    var n = nodes[drag];
    n.x = ((ev.clientX - box.left) / box.width) * W;
    n.y = ((ev.clientY - box.top) / box.height) * H;
    n.vx = 0;
    n.vy = 0;
    wake();
  });

  function release() {
    if (drag >= 0) {
      nodes[drag].fixed = false;
      drag = -1;
      wake();
    }
  }
  svg.addEventListener("pointerup", release);
  svg.addEventListener("pointercancel", release);

  // Seed the layout, then let it settle before the first paint.
  for (var settle = 0; settle < 200; settle++) {
    step();
  }
  draw();
  staticSvg.replaceWith(svg);
  wake();
})();
