import {div, img, span,
  e, Store, Component} from './base'

class App extends Component {
  render(state) {
    return div({}, [
      ...state.entries.slice(state.offset, 100).map((entry) => {
        return e(Entry, entry);
      }),
    ]);
  }
}

class Entry extends Component {
  render(state) {
    return div({}, [
      ...state.images.map(image => {
        return img({
          style: {
            maxWidth: '640px',
            border: '1px solid grey',
            padding: '5px',
          },
          src: image,
        });
      }),
      ...state.goods.map(good => {
        console.log(good);
        return div({}, [
          span({}, '￥' + good.price),
          e('a', {
            href: 'http://www.vvic.com/item.html?id=' + good.good_id,
            target: '_blank',
          }, 'http://www.vvic.com/item.html?id=' + good.good_id),
        ]);
      }),
    ]);
  }
}

let initState = {
  entries: window.data,
  offset: 0,
};
let app = new App(initState);
app.bind(document.getElementById('app'));
let store = new Store(initState);
app.setStore(store);
let emit = store.emit.bind(store);
