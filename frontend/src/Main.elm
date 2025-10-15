module Main exposing (main)

import Browser
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Attributes as Attrs
import Html.Events exposing (onClick, onInput)
import Http
import Json.Decode as D
import Json.Encode as E


-- CONFIG

apiBase : String
apiBase =
    "http://localhost:8080"


-- TYPES

type alias Game =
    { id : Int
    , sport : String
    , home : String
    , away : String
    , start_time : String
    , status : String
    , home_pool_tokens : Int
    , away_pool_tokens : Int
    , draw_pool_tokens : Int
    , home_odds : Float
    , away_odds : Float
    , draw_odds : Float
    }

type Page
    = ListPage
    | DetailPage Int

type alias Model =
    { page : Page
    , games : List Game
    , selected : Maybe Game
    , stake : String
    , selection : String
    , userId : Int
    , error : Maybe String
    }

type Msg
    = GotGames (Result Http.Error (List Game))
    | GotGame (Result Http.Error Game)
    | GoList
    | GoDetail Int
    | SetStake String
    | SetSelection String
    | PlaceBet
    | BetDone (Result Http.Error ())
    | ClearError


-- INIT

main : Program () Model Msg
main =
    Browser.element
        { init = \_ -> ( initModel, fetchGames )
        , update = update
        , view = view
        , subscriptions = \_ -> Sub.none
        }

initModel : Model
initModel =
    { page = ListPage
    , games = []
    , selected = Nothing
    , stake = "10"
    , selection = "home"
    , userId = 1
    , error = Nothing
    }


-- UPDATE

update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotGames (Ok gs) ->
            ( { model | games = gs }, Cmd.none )

        GotGames (Err e) ->
            ( { model | error = Just (errToString e) }, Cmd.none )

        GotGame (Ok g) ->
            ( { model | selected = Just g }, Cmd.none )

        GotGame (Err e) ->
            ( { model | error = Just (errToString e) }, Cmd.none )

        GoList ->
            ( { model | page = ListPage, selected = Nothing }, fetchGames )

        GoDetail gid ->
            ( { model | page = DetailPage gid, selected = Nothing }, fetchGame gid )

        SetStake s ->
            ( { model | stake = s }, Cmd.none )

        SetSelection s ->
            ( { model | selection = s }, Cmd.none )

        PlaceBet ->
            case ( model.page, model.selected ) of
                ( DetailPage gid, Just _ ) ->
                    let
                        stakeInt =
                            String.toInt model.stake |> Maybe.withDefault 0

                        body =
                            E.object
                                [ ( "user_id", E.int model.userId )
                                , ( "selection", E.string model.selection )
                                , ( "stake", E.int stakeInt )
                                ]
                    in
                    ( model
                    , Http.request
                        { method = "POST"
                        , headers = []
                        , url = apiBase ++ "/api/games/" ++ String.fromInt gid ++ "/bets"
                        , body = Http.jsonBody body
                        , expect = Http.expectWhatever (\_ -> BetDone (Ok ()))
                        , timeout = Nothing
                        , tracker = Nothing
                        }
                    )

                _ ->
                    ( model, Cmd.none )

        BetDone (Ok _) ->
            case model.page of
                DetailPage gid ->
                    ( model, fetchGame gid )

                _ ->
                    ( model, Cmd.none )

        BetDone (Err e) ->
            ( { model | error = Just (errToString e) }, Cmd.none )

        ClearError ->
            ( { model | error = Nothing }, Cmd.none )


-- HTTP

fetchGames : Cmd Msg
fetchGames =
    Http.get
        { url = apiBase ++ "/api/games"
        , expect = Http.expectJson GotGames (D.list gameDecoder)
        }

fetchGame : Int -> Cmd Msg
fetchGame gid =
    Http.get
        { url = apiBase ++ "/api/games/" ++ String.fromInt gid
        , expect = Http.expectJson GotGame gameDecoder
        }


-- DECODERS

gameDecoder : D.Decoder Game
gameDecoder =
    -- Decode first 6 fields
    D.map6
        (\id sport home away start status ->
            { id = id
            , sport = sport
            , home = home
            , away = away
            , start_time = start
            , status = status
            , home_pool_tokens = 0
            , away_pool_tokens = 0
            , draw_pool_tokens = 0
            , home_odds = 0
            , away_odds = 0
            , draw_odds = 0
            }
        )
        (D.field "id" D.int)
        (D.field "sport" D.string)
        (D.field "home" D.string)
        (D.field "away" D.string)
        (D.field "start_time" D.string)
        (D.field "status" D.string)
        |> D.andThen
            (\partial ->
                -- Then decode remaining 6 fields and merge
                D.map6
                    (\hp ap dp ho ao d0 ->
                        { partial
                            | home_pool_tokens = hp
                            , away_pool_tokens = ap
                            , draw_pool_tokens = dp
                            , home_odds = ho
                            , away_odds = ao
                            , draw_odds = d0
                        }
                    )
                    (D.field "home_pool_tokens" D.int)
                    (D.field "away_pool_tokens" D.int)
                    (D.field "draw_pool_tokens" D.int)
                    (D.field "home_odds" D.float)
                    (D.field "away_odds" D.float)
                    (D.field "draw_odds" D.float)
            )


-- VIEW

view : Model -> Html Msg
view model =
    div []
        [ h2 [] [ text "IMPredict (tokens only • demo)" ]
        , case model.error of
            Just e ->
                div [ class "card" ]
                    [ text e
                    , button [ onClick ClearError, style "margin-left" "8px" ] [ text "ok" ]
                    ]

            Nothing ->
                text ""
        , case model.page of
            ListPage ->
                viewList model.games

            DetailPage gid ->
                viewDetail gid model
        ]


viewList : List Game -> Html Msg
viewList games =
    div []
        [ h3 [] [ text "Games" ]
        , button [ onClick GoList ] [ text "Refresh" ]
        , div []
            (List.map
                (\g ->
                    div [ class "card" ]
                        [ div [] [ strong [] [ text (g.sport ++ ": " ++ g.home ++ " vs " ++ g.away) ] ]
                        , div [ class "muted" ] [ text ("Starts: " ++ g.start_time) ]
                        , div [] [ text ("Status: " ++ g.status) ]
                        , div [] [ text ("Pools H/A/D: "
                                ++ String.fromInt g.home_pool_tokens ++ " / "
                                ++ String.fromInt g.away_pool_tokens ++ " / "
                                ++ String.fromInt g.draw_pool_tokens) ]
                        , div [] [ text ("Implied odds H/A/D: "
                                ++ pct g.home_odds ++ " / "
                                ++ pct g.away_odds ++ " / "
                                ++ pct g.draw_odds) ]
                        , button [ onClick (GoDetail g.id), style "margin-top" "6px" ] [ text "Open" ]
                        ]
                )
                games
            )
        ]


viewDetail : Int -> Model -> Html Msg
viewDetail gid model =
    case model.selected of
        Nothing ->
            div [] [ button [ onClick GoList ] [ text "← Back" ], text " Loading..." ]

        Just g ->
            div []
                [ button [ onClick GoList ] [ text "← Back" ]
                , h3 [] [ text (g.sport ++ ": " ++ g.home ++ " vs " ++ g.away) ]
                , div [] [ text ("Status: " ++ g.status) ]
                , div [] [ text ("Pools H/A/D: "
                        ++ String.fromInt g.home_pool_tokens ++ " / "
                        ++ String.fromInt g.away_pool_tokens ++ " / "
                        ++ String.fromInt g.draw_pool_tokens) ]
                , div [] [ text ("Implied odds H/A/D: "
                        ++ pct g.home_odds ++ " / "
                        ++ pct g.away_odds ++ " / "
                        ++ pct g.draw_odds) ]
                , div [ style "margin-top" "10px" ]
                    [ label [] [ text "Pick: " ]
                    , select [ onInput SetSelection ]
                        [ option [ value "home", selected (model.selection == "home") ] [ text "Home" ]
                        , option [ value "away", selected (model.selection == "away") ] [ text "Away" ]
                        , option [ value "draw", selected (model.selection == "draw") ] [ text "Draw" ]
                        ]
                    , label [ style "margin-left" "8px" ] [ text "Stake: " ]
                    , input
                        [ type_ "number"
                        , value model.stake
                        , onInput SetStake
                        , Attrs.min "1"  -- disambiguate `min`
                        ]
                        []
                    , button [ onClick PlaceBet, style "margin-left" "8px" ] [ text "Place Bet" ]
                    ]
                ]


pct : Float -> String
pct x =
    let
        v =
            (toFloat (round (x * 1000))) / 10
    in
    String.fromFloat v ++ "%"


-- HELPERS

errToString : Http.Error -> String
errToString e =
    case e of
        Http.BadUrl u ->
            "Bad URL: " ++ u

        Http.Timeout ->
            "Timeout"

        Http.NetworkError ->
            "Network error"

        Http.BadStatus s ->
            "HTTP " ++ String.fromInt s

        Http.BadBody _ ->
            "Bad body"
